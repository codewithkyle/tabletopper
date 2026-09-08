// Package tiling is the background half of a map upload: it turns the original
// a request stored into the tile pyramid the table renders from.
//
// IT IS A JOB RATHER THAN A HANDLER because of what tiling one map costs. A
// 108-megapixel PNG is 432 MB the moment it is decoded, the pyramid on top of
// it is around 144 megapixels of WebP to encode, and the tiles are hundreds of
// PUTs. None of that belongs inside a request, and a request that tried would
// hold a connection open for a minute to do it.
//
// EXACTLY ONE MAP IS TILED AT A TIME, in one process. That is not a throughput
// decision, it is a memory one: two of those decodes at once is most of a small
// box, and serialising them is what makes the ceiling a number the box can be
// sized against rather than a function of how many people uploaded at once.
//
// THE ROW IS THE LEDGER FOR WHAT THE BUCKET HOLDS, here as everywhere else in
// this app, and a pyramid is under a prefix named by a ULID. Every prefix in
// the bucket is named by some row: as tile_gen while it is serving, or as
// tile_lease while it is being built. Recovery is what keeps the second half
// true across a crash -- a process that dies mid-build leaves a prefix that
// only its row's lease remembers, and reclaiming the row is what finds it
// again. Nothing here writes a prefix that no column names, and nothing clears
// a column until the prefix it named is gone.
package tiling

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"

	"github.com/oklog/ulid/v2"
)

const (
	// DefaultTileSize is what a map is tiled at when its row does not say.
	// 512 rather than the 256 web maps default to: at 512 a 12000x9000 map is
	// 584 objects instead of 2276, which is a quarter of the PUTs on upload
	// and a quarter of the requests on a pan, and it is nowhere near any GPU's
	// texture limit. It is written to the row so that changing it later
	// re-tiles from the retained original instead of needing a re-upload.
	DefaultTileSize = 512

	// interval is how long a pass waits before looking for more work. It is
	// short because it is the delay before a freshly uploaded map starts
	// tiling, and the card on the assets page is polling every two seconds
	// meanwhile; the query it costs is one indexed UPDATE that matches
	// nothing.
	interval = 5 * time.Second

	// leaseWindow is how long a claim is honoured before another pass may
	// take it back. A job has to finish well inside it -- reclaiming a live
	// job would delete the tiles it is still writing -- and tiling the
	// largest map this app accepts is an order of magnitude under it.
	leaseWindow = 15 * time.Minute

	// MaxAttempts is how many times a map is tiled before it is left alone.
	// A transient R2 outage gets three tries a lease window apart; an image
	// that cannot be decoded stops rather than being retried until the
	// process is restarted.
	//
	// IT IS EXPORTED BECAUSE THE CARD HAS TO SAY WHICH OF THOSE HAPPENED. A map
	// that has failed once is going to be tried again in a few minutes and a map
	// that has failed three times is not, and a card that said "Tiling gave up"
	// to both was telling the first one something untrue -- which is how a game
	// master ends up pressing a button they did not need. The controllers
	// compare a row's tile_attempts against this to decide which sentence the
	// card carries; a manual retry sets the count back to zero, so pressing it
	// buys three more automatic tries as well as an immediate one.
	MaxAttempts = 3

	// strandedBatch is the LIMIT inside ListStrandedTilingJobs, which sqlc
	// gives no parameter for. It is repeated here because a pass that came
	// back full has more waiting, and the two have to be kept in step.
	strandedBatch = 100
)

// rows is the part of *queries.Queries this package uses, and bucket the part
// of *storage.Client. They are interfaces so that the ordering rules below --
// which prefix is deleted before which column is cleared, and what happens when
// one of those fails -- can be exercised in a test. Nothing else about them is
// abstract: the concrete types are what run.
type rows interface {
	ClaimMapForTiling(ctx context.Context, tileLease *ulid.ULID) (sql.Result, error)
	GetLeasedMap(ctx context.Context, tileLease *ulid.ULID) (queries.Asset, error)
	CompleteMapTiling(ctx context.Context, arg queries.CompleteMapTilingParams) (sql.Result, error)
	FailMapTiling(ctx context.Context, arg queries.FailMapTilingParams) (sql.Result, error)
	ListStrandedTilingJobs(ctx context.Context, tileLeasedAt sql.NullTime) ([]queries.ListStrandedTilingJobsRow, error)
	RequeueStrandedTilingJob(ctx context.Context, arg queries.RequeueStrandedTilingJobParams) (sql.Result, error)
	RequeueFailedTilingJobs(ctx context.Context, arg queries.RequeueFailedTilingJobsParams) (sql.Result, error)
}

type bucket interface {
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Put(ctx context.Context, key string, body []byte, contentType string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

type worker struct {
	db     rows
	bucket bucket

	// reclaimedAt is when stranded and failed rows were last swept back to
	// pending. Nothing can be found by that sweep sooner than a lease window
	// after it went wrong, so running it on every pass would be the same
	// three queries every five seconds to answer a question whose answer
	// cannot have changed.
	reclaimedAt time.Time
}

// Maps tiles uploaded maps in the background until ctx is cancelled, once at
// start and then on a ticker -- the shape the sweepers next door use, for the
// same reason: a process that has just come up should not wait for the interval
// to do the work it was down for.
func Maps(ctx context.Context, q *queries.Queries, store *storage.Client) {
	w := &worker{db: q, bucket: store}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		w.pass(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Map tiling stopped")
				return
			case <-ticker.C:
				w.pass(ctx)
			}
		}
	}()
}

// pass reclaims what an earlier pass abandoned and then tiles every map that is
// waiting, one at a time, until there are none left.
func (w *worker) pass(ctx context.Context) {
	if time.Since(w.reclaimedAt) >= leaseWindow {
		w.reclaim(ctx)
		w.reclaimedAt = time.Now()
	}

	for ctx.Err() == nil {
		asset, ok := w.claim(ctx)
		if !ok {
			return
		}
		w.tile(ctx, asset)
	}
}

// claim takes the oldest waiting map and reads back what it needs to tile it.
//
// The lease is a fresh ULID and it does two jobs: it is the token that proves
// the claim, since a conditional UPDATE plus a SELECT on the token is how a row
// is taken and found again without RETURNING, and it is the generation the job
// writes under. One value naming both the claim and the prefix it produces is
// what lets recovery find an abandoned job's tiles from its row alone.
func (w *worker) claim(ctx context.Context) (queries.Asset, bool) {
	lease := ulid.Make()

	result, err := w.db.ClaimMapForTiling(ctx, &lease)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Failed to claim a map for tiling", "error", err)
		}
		return queries.Asset{}, false
	}
	taken, err := result.RowsAffected()
	if err != nil {
		// The claim may have landed. The row keeps whatever it got, and
		// reclaim takes it back a lease window from now if it did.
		slog.Error("Failed to read back a tiling claim", "error", err, "lease", lease.String())
		return queries.Asset{}, false
	}
	if taken == 0 {
		return queries.Asset{}, false
	}

	asset, err := w.db.GetLeasedMap(ctx, &lease)
	if err != nil {
		// The row is claimed and this process has lost track of it. It keeps
		// the lease and the timestamp, so reclaim takes it back a lease
		// window from now; nothing was written to the bucket under it yet.
		if ctx.Err() == nil {
			slog.Error("Failed to read back a claimed map", "error", err, "lease", lease.String())
		}
		return queries.Asset{}, false
	}

	return asset, true
}

// reclaim puts back what an earlier pass left behind: rows stuck working
// because the process holding them died, and rows that failed and have tries
// left.
func (w *worker) reclaim(ctx context.Context) {
	cutoff := sql.NullTime{Time: time.Now().Add(-leaseWindow), Valid: true}

	stranded, err := w.db.ListStrandedTilingJobs(ctx, cutoff)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Failed to list stranded tiling jobs", "error", err)
		}
		return
	}
	if len(stranded) == strandedBatch {
		slog.Warn("Reclaiming a full batch of stranded tiling jobs", "count", len(stranded))
	}

	for _, job := range stranded {
		if ctx.Err() != nil {
			return
		}
		if job.TileLease == nil {
			// working with no lease cannot be produced by anything here, and
			// there is no prefix to name, so the row is simply put back.
			slog.Warn("A tiling job was working with no lease", "assetID", job.ID.String())
		} else if err := w.bucket.DeletePrefix(ctx, storage.MapGenerationPrefix(job.OwnerID, job.ID, *job.TileLease)); err != nil {
			// The row keeps its lease, which is the only record that the
			// half-built prefix exists. Clearing it here would strand the
			// objects instead of the row.
			if ctx.Err() == nil {
				slog.Error("Failed to delete a stranded generation", "error", err, "assetID", job.ID.String())
			}
			continue
		}

		_, err := w.db.RequeueStrandedTilingJob(ctx, queries.RequeueStrandedTilingJobParams{
			ID:        job.ID,
			TileLease: job.TileLease,
		})
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("Failed to requeue a stranded tiling job", "error", err, "assetID", job.ID.String())
			}
			continue
		}
		slog.Info("Requeued a stranded tiling job", "assetID", job.ID.String())
	}

	result, err := w.db.RequeueFailedTilingJobs(ctx, queries.RequeueFailedTilingJobsParams{
		TileAttempts: MaxAttempts,
		TileLeasedAt: cutoff,
	})
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Failed to requeue failed tiling jobs", "error", err)
		}
		return
	}
	if retrying, err := result.RowsAffected(); err == nil && retrying > 0 {
		slog.Info("Requeued failed tiling jobs", "count", retrying)
	}
}
