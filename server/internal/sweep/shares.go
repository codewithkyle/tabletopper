package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
)

const (
	expiredShareInterval = time.Hour
	expiredShareGrace    = 24 * time.Hour
)

func ExpiredShares(ctx context.Context, q *queries.Queries) {
	Every(ctx, "Expired share sweep", expiredShareInterval, func(ctx context.Context) {
		sweepExpiredShares(ctx, q)
	})
}
func sweepExpiredShares(ctx context.Context, q *queries.Queries) {
	cutoff := sql.NullTime{Time: time.Now().Add(-expiredShareGrace), Valid: true}
	result, err := q.DeleteExpiredShares(ctx, cutoff)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		slog.Error("Failed to delete expired shares", "error", err)
		return
	}
	if rows, err := result.RowsAffected(); err == nil && rows > 0 {
		slog.Info("Swept expired shares", "count", rows)
	}
}
