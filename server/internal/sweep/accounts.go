package sweep

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"

	"github.com/oklog/ulid/v2"
)

const deletedAccountInterval = 5 * time.Minute

type Tables interface {
	Close(ctx context.Context, roomID ulid.ULID)
	ForgetCharacter(ctx context.Context, character ulid.ULID)
	ForgetAsset(ctx context.Context, asset ulid.ULID)
}
type Objects interface {
	DeletePrefix(ctx context.Context, prefix string) error
}
type Accounts struct {
	q      *queries.Queries
	store  Objects
	tables Tables
	wake   chan struct{}
}

func DeletedAccounts(ctx context.Context, q *queries.Queries, store Objects, tables Tables) *Accounts {
	a := &Accounts{q: q, store: store, tables: tables, wake: make(chan struct{}, 1)}
	go func() {
		ticker := time.NewTicker(deletedAccountInterval)
		defer ticker.Stop()
		a.sweep(ctx)
		for {
			select {
			case <-ctx.Done():
				slog.Info("Deleted account sweep stopped")
				return
			case <-a.wake:
				a.sweep(ctx)
			case <-ticker.C:
				a.sweep(ctx)
			}
		}
	}()
	return a
}
func (a *Accounts) Wake() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}
func (a *Accounts) sweep(ctx context.Context) {
	users, err := a.q.ListDeletedUsers(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		slog.Error("Failed to list deleted accounts", "error", err)
		return
	}
	for _, user := range users {
		if err := PurgeAccount(ctx, a.q, a.store, a.tables, user); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("Failed to purge a deleted account; it will be tried again", "error", err, "userID", user.String())
			continue
		}
		slog.Info("Purged a deleted account", "userID", user.String())
	}
}
func PurgeAccount(ctx context.Context, q *queries.Queries, store Objects, tables Tables, user ulid.ULID) error {
	rooms, err := q.ListOwnedRoomIDs(ctx, user)
	if err != nil {
		return fmt.Errorf("rooms: %w", err)
	}
	for _, roomID := range rooms {
		tables.Close(ctx, roomID)
	}
	if _, err := q.ClearSessionsInOwnedRooms(ctx, user); err != nil {
		return fmt.Errorf("room sessions: %w", err)
	}
	if _, err := q.DeleteOwnedRooms(ctx, user); err != nil {
		return fmt.Errorf("rooms: %w", err)
	}
	characters, err := q.ListOwnedCharacterIDs(ctx, user)
	if err != nil {
		return fmt.Errorf("characters: %w", err)
	}
	for _, character := range characters {
		tables.ForgetCharacter(ctx, character)
	}
	steps := []struct {
		name string
		run  func(context.Context, ulid.ULID) (sql.Result, error)
	}{
		{"spells", q.DeleteOwnedSpells},
		{"spell slots", q.DeleteOwnedSpellSlots},
		{"inventory", q.DeleteOwnedInventory},
		{"attacks", q.DeleteOwnedAttacks},
		{"journals", q.DeleteOwnedJournals},
		{"shares", q.DeleteOwnedShares},
		{"characters", q.DeleteOwnedCharacters},
		{"monster actions", q.DeleteOwnedMonsterActions},
		{"monsters", q.DeleteOwnedMonsters},
		{"scenes", q.DeleteOwnedScenes},
	}
	for _, step := range steps {
		if _, err := step.run(ctx, user); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	assets, err := q.ListOwnedAssetIDs(ctx, user)
	if err != nil {
		return fmt.Errorf("assets: %w", err)
	}
	for _, asset := range assets {
		tables.ForgetAsset(ctx, asset)
	}
	if err := store.DeletePrefix(ctx, storage.UserPrefix(user)); err != nil {
		return fmt.Errorf("objects: %w", err)
	}
	if _, err := q.DeleteOwnedAssets(ctx, user); err != nil {
		return fmt.Errorf("assets: %w", err)
	}
	if _, err := q.DeleteUserSessions(ctx, user); err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	if _, err := q.DeleteUser(ctx, user); err != nil {
		return fmt.Errorf("user: %w", err)
	}
	return nil
}
