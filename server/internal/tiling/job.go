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

const encoders = 16

var errAbandoned = errors.New("tiling: abandoned")

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
		if ctx.Err() == nil {
			slog.Error("Failed to publish a tiled map", "error", err, "assetID", asset.ID.String())
		}
		return
	}
	published, err := completed.RowsAffected()
	if err != nil {
		slog.Error("Failed to read back whether a tiled map was published", "error", err, "assetID", asset.ID.String())
		return
	}
	if published == 0 {
		w.discard(ctx, asset, generation, "the map was replaced while it was being tiled")
		return
	}
	if asset.TileGen != nil {
		superseded := storage.MapGenerationPrefix(asset.OwnerID, asset.ID, *asset.TileGen)
		if err := w.bucket.DeletePrefix(ctx, superseded); err != nil {
			slog.Error("Failed to delete the superseded generation; it is orphaned",
				"error", err, "assetID", asset.ID.String(), "prefix", superseded)
		}
	}
}
func (w *worker) abandon(ctx context.Context, asset queries.Asset, generation ulid.ULID, cause error) {
	if ctx.Err() != nil {
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
