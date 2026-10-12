package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
)

const (
	journalImageInterval = time.Hour
	journalImageGrace    = 24 * time.Hour
	journalImageBatch    = 100
)

func JournalImages(ctx context.Context, db *sql.DB) {
	go func() {
		ticker := time.NewTicker(journalImageInterval)
		defer ticker.Stop()
		sweepJournalImages(ctx, db)
		for {
			select {
			case <-ctx.Done():
				slog.Info("Journal image sweep stopped")
				return
			case <-ticker.C:
				sweepJournalImages(ctx, db)
			}
		}
	}()
}
func sweepJournalImages(ctx context.Context, db *sql.DB) {
	q := queries.New(db)
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
		deleted := int64(0)
		for _, image := range batch {
			var gone int64
			err := inTx(ctx, db, func(q *queries.Queries) error {
				result, err := q.DeleteSweptJournalImage(ctx, queries.DeleteSweptJournalImageParams{
					ID:     image.ID,
					Cutoff: cutoff,
				})
				if err != nil {
					return err
				}
				gone, err = result.RowsAffected()
				if err != nil || gone == 0 {
					return err
				}
				return Purge(ctx, q, Key(image.FilePath))
			})
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Error("Failed to sweep a journal image", "error", err, "assetID", image.ID.String())
				continue
			}
			deleted += gone
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
