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
	expiredShareGrace = 24 * time.Hour
)
func ExpiredShares(ctx context.Context, q *queries.Queries) {
	go func() {
		ticker := time.NewTicker(expiredShareInterval)
		defer ticker.Stop()
		sweepExpiredShares(ctx, q)
		for {
			select {
			case <-ctx.Done():
				slog.Info("Expired share sweep stopped")
				return
			case <-ticker.C:
				sweepExpiredShares(ctx, q)
			}
		}
	}()
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
