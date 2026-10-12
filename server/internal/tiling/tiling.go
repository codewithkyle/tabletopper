package tiling

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
	"tabletopper/internal/sweep"

	"github.com/oklog/ulid/v2"
)

const (
	DefaultTileSize = 512
	interval        = 5 * time.Second
	leaseWindow     = 15 * time.Minute
	MaxAttempts     = 3
	strandedBatch   = 100
	requeueBudget   = 10 * time.Second
)

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
	db          rows
	bucket      bucket
	reclaimedAt time.Time
}
type Worker struct {
	loop *sweep.Loop
}

func Maps(ctx context.Context, q *queries.Queries, store *storage.Client) *Worker {
	return start(ctx, &worker{db: q, bucket: store})
}
func start(ctx context.Context, w *worker) *Worker {
	return &Worker{loop: sweep.Every(ctx, "Map tiling", interval, w.pass)}
}
func (w *Worker) Shutdown(ctx context.Context) {
	select {
	case <-w.loop.Done():
	case <-ctx.Done():
		slog.Warn("Map tiling was still stopping when the shutdown budget ran out")
	}
}
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
		slog.Error("Failed to read back a tiling claim", "error", err, "lease", lease.String())
		return queries.Asset{}, false
	}
	if taken == 0 {
		return queries.Asset{}, false
	}
	asset, err := w.db.GetLeasedMap(ctx, &lease)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("Failed to read back a claimed map", "error", err, "lease", lease.String())
		}
		return queries.Asset{}, false
	}
	return asset, true
}
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
			slog.Warn("A tiling job was working with no lease", "assetID", job.ID.String())
		} else if err := w.bucket.DeletePrefix(ctx, storage.MapGenerationPrefix(job.OwnerID, job.ID, *job.TileLease)); err != nil {
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
