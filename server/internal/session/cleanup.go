package session

import (
	"context"
	"log/slog"
	"time"

	"tabletopper/internal/sweep"
)

const (
	cleanupInterval = time.Hour
	cleanupGrace    = 24 * time.Hour
)

func (s *Store) StartCleanup(ctx context.Context) {
	sweep.Every(ctx, "Session cleanup", cleanupInterval, s.deleteExpired)
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
