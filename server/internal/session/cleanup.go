package session

import (
	"context"
	"log/slog"
	"time"
)

const (
	
	cleanupInterval = time.Hour

	
	
	cleanupGrace = 24 * time.Hour
)






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
