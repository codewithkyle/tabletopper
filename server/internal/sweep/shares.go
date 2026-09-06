package sweep

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
)

const (
	// expiredShareInterval is how often expired share rows are deleted.
	expiredShareInterval = time.Hour

	// expiredShareGrace is how long an expired share is kept after it stops
	// working. Nothing depends on the row being gone -- every read already
	// refuses an expired share -- so the only question the grace answers is
	// what the owner sees when they open the dialog the next morning: a link
	// that expired, which they can revoke and replace, rather than an entry
	// that appears never to have been shared at all.
	expiredShareGrace = 24 * time.Hour
)

// ExpiredShares deletes expired share rows in the background until ctx is
// cancelled, once at start and then hourly. It is the same shape as
// JournalImages next door and is a great deal less interesting: a share owns
// nothing in R2, so this is one statement and no ledger to keep in order.
//
// IT IS NOT WHAT ENFORCES EXPIRY. GetShareByToken carries the expiry in its
// WHERE clause, so a link stops working on the second it expires whether this
// has run or not; a sweeper that fell over would cost disk and nothing else.
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
