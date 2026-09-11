






















package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
)

const (
	
	journalImageInterval = time.Hour

	
	
	
	journalImageGrace = 24 * time.Hour

	
	
	
	
	journalImageBatch = 100
)





func JournalImages(ctx context.Context, q *queries.Queries, store *storage.Client) {
	go func() {
		ticker := time.NewTicker(journalImageInterval)
		defer ticker.Stop()

		sweepJournalImages(ctx, q, store)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Journal image sweep stopped")
				return
			case <-ticker.C:
				sweepJournalImages(ctx, q, store)
			}
		}
	}()
}








func sweepJournalImages(ctx context.Context, q *queries.Queries, store *storage.Client) {
	cutoff := sql.NullTime{Time: time.Now().Add(-journalImageGrace), Valid: true}

	swept := int64(0)
	for {
		batch, err := q.ListSweepableJournalImages(ctx, cutoff)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to list sweepable journal images", "error", err)
			return
		}
		if len(batch) == 0 {
			break
		}

		keys := make([]string, 0, len(batch))
		for _, image := range batch {
			keys = append(keys, image.FilePath)
		}
		
		
		
		
		if err := store.DeleteMany(ctx, keys); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to delete journal image objects", "error", err, "count", len(keys))
			return
		}

		deleted := int64(0)
		for _, image := range batch {
			result, err := q.DeleteSweptJournalImage(ctx, queries.DeleteSweptJournalImageParams{
				ID:     image.ID,
				Cutoff: cutoff,
			})
			if err != nil {
				slog.Error("Failed to delete swept journal image row", "error", err, "assetID", image.ID.String())
				continue
			}
			if rows, err := result.RowsAffected(); err == nil {
				deleted += rows
			}
		}
		swept += deleted

		
		
		
		
		
		if deleted == 0 || len(batch) < journalImageBatch {
			break
		}
	}

	if swept > 0 {
		slog.Info("Swept journal images", "count", swept)
	}
}
