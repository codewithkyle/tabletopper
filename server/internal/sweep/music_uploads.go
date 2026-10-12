package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
)

const (
	musicUploadInterval = time.Hour
	musicUploadGrace    = 6 * time.Hour
	musicUploadBatch    = 100
)

func MusicUploads(ctx context.Context, db *sql.DB) {
	go func() {
		ticker := time.NewTicker(musicUploadInterval)
		defer ticker.Stop()
		sweepMusicUploads(ctx, db)
		for {
			select {
			case <-ctx.Done():
				slog.Info("Abandoned music upload sweep stopped")
				return
			case <-ticker.C:
				sweepMusicUploads(ctx, db)
			}
		}
	}()
}
func sweepMusicUploads(ctx context.Context, db *sql.DB) {
	q := queries.New(db)
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
		failed := 0
		for _, row := range rows {
			err := inTx(ctx, db, func(q *queries.Queries) error {
				if err := q.DeleteAsset(ctx, queries.DeleteAssetParams{ID: row.ID, OwnerID: row.OwnerID}); err != nil {
					return err
				}
				return Purge(ctx, q, Key(row.FilePath))
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				failed++
				slog.Error("Failed to sweep an abandoned track", "error", err, "assetID", row.ID.String())
			}
		}
		slog.Info("Swept abandoned music uploads", "count", len(rows)-failed)
		if failed > 0 || len(rows) < musicUploadBatch {
			return
		}
	}
}
