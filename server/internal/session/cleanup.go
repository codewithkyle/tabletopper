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
// cancelled. What it collects is rows that ran out on their own -- a session
// left open on a machine nobody came back to -- plus the ones /authorize and
// Logout ended early, which are expired the moment they are ended and sit
// through the grace below like any other.
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
