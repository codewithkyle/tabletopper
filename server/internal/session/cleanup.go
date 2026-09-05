package session

import (
	"context"
	"log/slog"
	"time"
)

const (
	// cleanupInterval is how often expired sessions are swept.
	cleanupInterval = time.Hour

	// cleanupGrace keeps expired rows around briefly so an expiry can still be
	// inspected after the fact.
	cleanupGrace = 24 * time.Hour
)

// StartCleanup sweeps expired sessions in the background until ctx is
// cancelled. It also clears the rows left behind by repeat logins, since
// /authorize inserts a new session every time it runs.
func (s *Store) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		s.deleteExpired(ctx)

		for {
			select {
			case <-ctx.Done():
				slog.Info("Session cleanup stopped")
				return
			case <-ticker.C:
				s.deleteExpired(ctx)
			}
		}
	}()
}

func (s *Store) deleteExpired(ctx context.Context) {
	result, err := s.q.DeleteExpiredSessions(ctx, time.Now().Add(-cleanupGrace))
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
