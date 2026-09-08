package tiling

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"sync"
	"time"

	"tabletopper/internal/images"
	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
	"tabletopper/internal/tiler"

	"github.com/oklog/ulid/v2"
)

// encoders is how many tiles are encoded and uploaded at once.
//
// ONE POOL DOES BOTH JOBS ON PURPOSE. Encoding is where the seconds go -- a
// full pyramid is around 144 megapixels of WebP through libwebp -- and the PUTs
// are almost entirely latency. A pool that only uploaded would leave the encode
// serial behind it; a pool that only encoded would wait on R2 with idle cores.
// Sixteen is enough to keep both busy on any box this runs on, and it is also
// the bound on how far the tiler may run ahead: the tiler blocks handing over a
// tile once the pool is full, which is what stops a whole pyramid of decoded
// pixels accumulating in front of the encoders.
const encoders = 16

// errAbandoned is what the tiler is told when the pool has already failed. The
// pool's own error is the one that gets reported; this exists so the tiler
// stops walking the pyramid rather than finishing it for nothing.
var errAbandoned = errors.New("tiling: abandoned")

// tile builds one map's pyramid and then either publishes it or throws it away.
//
// NOTHING IS DELETED BEFORE THE NEW PYRAMID IS COMPLETE. The generation being
// built is at its own prefix, so the one that is serving keeps serving for the
// minute this takes; a map being replaced never has a window with no tiles.
func (w *worker) tile(ctx context.Context, asset queries.Asset) {
	if asset.TileLease == nil {
		slog.Error("A claimed map has no lease", "assetID", asset.ID.String())
		return
	}
	generation := *asset.TileLease

	started := time.Now()
	result, previewPath, err := w.build(ctx, asset, generation)
	if err != nil {
		w.abandon(ctx, asset, generation, err)
		return
	}

	w.publish(ctx, asset, generation, result, previewPath)
	slog.Info("Tiled a map",
		"assetID", asset.ID.String(),
		"size", fmt.Sprintf("%dx%d", result.Width, result.Height),
		"maxZoom", result.MaxZoom,
		"took", time.Since(started).Round(time.Millisecond),
	)
}

// build reads the original back out of the bucket and writes a whole
// generation: every tile of every level, and the preview.
//
// THE ORIGINAL IS READ FROM R2 RATHER THAN HANDED OVER BY THE REQUEST. A
// channel of decoded images would not survive a restart and would bound how
// many uploads may be in flight by this process's memory rather than by intent.
// Reading it back keeps the row as the only description of the work: the job's
// input is an object the row already names.
func (w *worker) build(ctx context.Context, asset queries.Asset, generation ulid.ULID) (tiler.Result, string, error) {
	original, _, err := w.bucket.Get(ctx, asset.FilePath)
	if err != nil {
		return tiler.Result{}, "", fmt.Errorf("reading the original: %w", err)
	}
	defer original.Close()

	tileSize := DefaultTileSize
	if asset.TileSize.Valid && asset.TileSize.Int16 > 0 {
		tileSize = int(asset.TileSize.Int16)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tiles := make(chan tiler.Tile, encoders)
	var wg sync.WaitGroup
	var once sync.Once
	var poolErr error

	for range encoders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// The range keeps draining after a failure rather than
			// returning, so a tiler still holding a tile is never blocked
			// handing it to a pool that has stopped reading.
			for tile := range tiles {
				if ctx.Err() != nil {
					continue
				}
				if err := w.writeTile(ctx, asset, generation, tile); err != nil {
					once.Do(func() {
						poolErr = err
						cancel()
					})
				}
			}
		}()
	}

	// THE LAST TILE EMITTED IS THE TOP OF THE PYRAMID, and the top level is a
	// single tile by the definition of max zoom -- it is the level at which
	// both axes fit inside one. That is what the preview is made from, rather
	// than the source: squaring a 108-megapixel image down to 256 would be a
	// second full-resolution resample for a thumbnail, when something already
	// about that size has just been built.
	var top *image.RGBA
	result, buildErr := tiler.Build(original, tileSize, func(tile tiler.Tile) error {
		if ctx.Err() != nil {
			return errAbandoned
		}
		top = tile.Image
		select {
		case tiles <- tile:
			return nil
		case <-ctx.Done():
			return errAbandoned
		}
	})

	close(tiles)
	wg.Wait()

	// The pool's error comes first: when it failed, what the tiler returned is
	// errAbandoned, which says only that it noticed.
	if poolErr != nil {
		return tiler.Result{}, "", poolErr
	}
	if buildErr != nil {
		return tiler.Result{}, "", buildErr
	}
	if top == nil {
		return tiler.Result{}, "", errors.New("the pyramid emitted no tiles")
	}

	preview, err := images.EncodeWebP(images.Square(top, images.MapPreviewSize))
	if err != nil {
		return tiler.Result{}, "", fmt.Errorf("encoding the preview: %w", err)
	}
	previewPath := storage.MapPreviewKey(asset.OwnerID, asset.ID, generation)
	if err := w.bucket.Put(ctx, previewPath, preview, "image/webp"); err != nil {
		return tiler.Result{}, "", fmt.Errorf("writing the preview: %w", err)
	}

	return result, previewPath, nil
}

// THE LEVELS ARE NOT WORTH THE SAME NUMBER, so the quality falls with the
// level rather than being one constant for the pyramid.
//
// Level 0 is the one a GM zooms all the way into, and it is the only level that
// still holds what the original held: whatever the encoder throws away there is
// thrown away for good, because every level above it was derived from these
// pixels and cannot put back what they lost. It gets 90.
//
// The top level is the whole map on one screen. It is already a box average of
// sixteen or a thousand source pixels, so its detail is gone before the encoder
// sees it and there is little left for artefacts to sit on -- and it is also
// the level fetched first, on every join, before anything is on screen at all.
// It gets 70, and the bytes saved come off the wait everybody sees.
//
// In between it is a straight line, which puts the middle of the pyramid at
// about 80. Nothing subtler is warranted: the ramp is a preference about detail
// against bytes, not a measurement, and a curve would only be a preference with
// more decimal places.
const (
	nativeQuality   = 90
	overviewQuality = 70
)

func tileQuality(z, maxZoom int) int {
	if maxZoom < 1 || z <= 0 {
		return nativeQuality
	}
	if z >= maxZoom {
		return overviewQuality
	}

	return nativeQuality - (nativeQuality-overviewQuality)*z/maxZoom
}

func (w *worker) writeTile(ctx context.Context, asset queries.Asset, generation ulid.ULID, tile tiler.Tile) error {
	encoded, err := images.EncodeWebPAt(tile.Image, tileQuality(tile.Z, tile.MaxZoom))
	if err != nil {
		return fmt.Errorf("encoding tile z%d %d,%d: %w", tile.Z, tile.X, tile.Y, err)
	}
	key := storage.MapTileKey(asset.OwnerID, asset.ID, generation, tile.Z, tile.X, tile.Y)
	if err := w.bucket.Put(ctx, key, encoded, "image/webp"); err != nil {
		return fmt.Errorf("writing tile z%d %d,%d: %w", tile.Z, tile.X, tile.Y, err)
	}
	return nil
}

// publish makes the finished generation the one that serves, and deletes the
// one it replaced.
func (w *worker) publish(ctx context.Context, asset queries.Asset, generation ulid.ULID, result tiler.Result, previewPath string) {
	completed, err := w.db.CompleteMapTiling(ctx, queries.CompleteMapTilingParams{
		TileGen:     &generation,
		Width:       sql.NullInt32{Int32: int32(result.Width), Valid: true},
		Height:      sql.NullInt32{Int32: int32(result.Height), Valid: true},
		TileSize:    sql.NullInt16{Int16: int16(result.TileSize), Valid: true},
		MaxZoom:     sql.NullInt16{Int16: int16(result.MaxZoom), Valid: true},
		PreviewPath: sql.NullString{String: previewPath, Valid: true},
		ID:          asset.ID,
		TileLease:   &generation,
	})
	if err != nil {
		// The row keeps the lease that names this prefix, so reclaim will
		// come back to it and the pyramid will be rebuilt rather than left
		// with nothing pointing at it.
		if ctx.Err() == nil {
			slog.Error("Failed to publish a tiled map", "error", err, "assetID", asset.ID.String())
		}
		return
	}

	published, err := completed.RowsAffected()
	if err != nil {
		// Which generation is serving is now unknown, so nothing is deleted:
		// the old prefix may still be the live one.
		slog.Error("Failed to read back whether a tiled map was published", "error", err, "assetID", asset.ID.String())
		return
	}
	if published == 0 {
		// THE CLAIM WAS TAKEN AWAY MID-BUILD: a replacement arrived, put the
		// row back to pending and overwrote the original. What was just built
		// describes a file that is gone, so it goes now rather than waiting
		// for something to notice it -- and the pending row is picked up on
		// the next turn of the loop.
		w.discard(ctx, asset, generation, "the map was replaced while it was being tiled")
		return
	}

	if asset.TileGen != nil {
		superseded := storage.MapGenerationPrefix(asset.OwnerID, asset.ID, *asset.TileGen)
		if err := w.bucket.DeletePrefix(ctx, superseded); err != nil {
			// The row no longer names this prefix, so nothing will find it
			// again. It is logged with the prefix because that is what a
			// hand cleanup needs, and it costs storage and nothing else.
			slog.Error("Failed to delete the superseded generation; it is orphaned",
				"error", err, "assetID", asset.ID.String(), "prefix", superseded)
		}
	}
}

// abandon throws away a generation that could not be finished and records the
// attempt against the row.
//
// THE PREFIX GOES BEFORE THE LEASE IS CLEARED, and if it cannot go, the lease
// is kept. A partial pyramid under a ULID nobody will use again is an orphan
// the moment the column naming it is cleared; leaving the row working means
// reclaim comes back for the same prefix a lease window later and tries again.
// It is the same objects-first, row-last order every other delete in this app
// uses, and the reason a retry writes a new generation rather than overwriting
// this one.
func (w *worker) abandon(ctx context.Context, asset queries.Asset, generation ulid.ULID, cause error) {
	if ctx.Err() != nil {
		// A shutdown, not a failure. The row keeps its claim and its lease,
		// and reclaim finishes the cleanup when the process comes back.
		slog.Info("Stopped tiling a map", "assetID", asset.ID.String())
		return
	}
	slog.Error("Failed to tile a map", "error", cause, "assetID", asset.ID.String())

	if !w.discard(ctx, asset, generation, "") {
		return
	}

	_, err := w.db.FailMapTiling(ctx, queries.FailMapTilingParams{
		ID:        asset.ID,
		TileLease: &generation,
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("Failed to record a failed tiling job", "error", err, "assetID", asset.ID.String())
	}
}

// discard deletes one generation's prefix, reporting whether it is gone.
func (w *worker) discard(ctx context.Context, asset queries.Asset, generation ulid.ULID, why string) bool {
	prefix := storage.MapGenerationPrefix(asset.OwnerID, asset.ID, generation)
	if err := w.bucket.DeletePrefix(ctx, prefix); err != nil {
		if ctx.Err() == nil {
			slog.Error("Failed to delete an abandoned generation", "error", err, "assetID", asset.ID.String(), "prefix", prefix)
		}
		return false
	}
	if why != "" {
		slog.Info("Discarded a tiled generation", "assetID", asset.ID.String(), "reason", why)
	}
	return true
}
