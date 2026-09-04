package session

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"main/internal/queries"
)

const (
	// cleanupInterval is how often expired sessions are swept.
	cleanupInterval = time.Hour

	// cleanupGrace keeps expired rows around briefly so an expiry can still be
	// inspected after the fact.
	cleanupGrace = 24 * time.Hour
)

// StartCleanup sweeps expired sessions until ctx is cancelled. It also clears
// the rows left behind by repeat logins, since /authorize inserts a new session
// every time it runs.
func StartCleanup(ctx context.Context, db *sql.DB) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		deleteExpired(ctx, db)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Session cleanup stopped")
				return
			case <-ticker.C:
				deleteExpired(ctx, db)
			}
		}
	}()
}

func deleteExpired(ctx context.Context, db *sql.DB) {
	q := queries.New(db)
	result, err := q.DeleteExpiredSessions(ctx, time.Now().Add(-cleanupGrace))
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		slog.Error("Failed to delete expired sessions", "error", err)
		return
	}

	if rows, err := result.RowsAffected(); err == nil && rows > 0 {
		slog.Info("Deleted expired sessions", "count", rows)
	}
}
