package sweep

import (
	"context"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
)

const (
	
	musicUploadInterval = time.Hour

	
	
	
	
	
	
	
	
	
	
	
	musicUploadGrace = 6 * time.Hour

	
	
	
	
	musicUploadBatch = 100
)

















func MusicUploads(ctx context.Context, q *queries.Queries, store *storage.Client) {
	go func() {
		ticker := time.NewTicker(musicUploadInterval)
		defer ticker.Stop()

		sweepMusicUploads(ctx, q, store)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Abandoned music upload sweep stopped")
				return
			case <-ticker.C:
				sweepMusicUploads(ctx, q, store)
			}
		}
	}()
}

func sweepMusicUploads(ctx context.Context, q *queries.Queries, store *storage.Client) {
	for {
		cutoff := time.Now().Add(-musicUploadGrace)

		rows, err := q.ListAbandonedMusicUploads(ctx, cutoff)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to list abandoned music uploads", "error", err)
			return
		}
		if len(rows) == 0 {
			return
		}

		for _, row := range rows {
			if err := store.Delete(ctx, row.FilePath); err != nil {
				if ctx.Err() != nil {
					return
				}
				
				
				
				slog.Error("Failed to delete an abandoned track; leaving its row", "error", err, "assetID", row.ID.String())
				continue
			}

			err := q.DeleteAsset(ctx, queries.DeleteAssetParams{ID: row.ID, OwnerID: row.OwnerID})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Failed to delete an abandoned track's row", "error", err, "assetID", row.ID.String())
			}
		}

		slog.Info("Swept abandoned music uploads", "count", len(rows))

		
		
		if len(rows) < musicUploadBatch {
			return
		}
	}
}
