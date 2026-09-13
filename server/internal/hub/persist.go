package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

var ErrNoRoom = errors.New("hub: no such open room")

type Store interface {
	Load(ctx context.Context, roomID ulid.ULID) (Loaded, error)
	Save(ctx context.Context, roomID ulid.ULID, snapshot []byte, seq uint64) error
	ClearMembership(ctx context.Context, roomID, userID ulid.ULID) error
	Preserve(ctx context.Context, roomID ulid.ULID, snapshot []byte) error
	AutosaveScene(ctx context.Context, roomID ulid.ULID, body []byte, preview *ulid.ULID) error
}
type Loaded struct {
	Name     string
	Locked   bool
	Snapshot json.RawMessage
}

func NewStore(q *queries.Queries) Store { return dbStore{q: q} }

type dbStore struct{ q *queries.Queries }

func (d dbStore) Load(ctx context.Context, roomID ulid.ULID) (Loaded, error) {
	row, err := d.q.GetRoomSnapshot(ctx, roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return Loaded{}, ErrNoRoom
	}
	if err != nil {
		return Loaded{}, fmt.Errorf("hub: load room: %w", err)
	}
	return Loaded{Name: row.Name, Locked: row.IsLocked, Snapshot: row.Snapshot}, nil
}
func (d dbStore) Save(ctx context.Context, roomID ulid.ULID, snapshot []byte, seq uint64) error {
	result, err := d.q.SaveRoomSnapshot(ctx, queries.SaveRoomSnapshotParams{
		Snapshot:    snapshot,
		SnapshotSeq: seq,
		ID:          roomID,
	})
	if err != nil {
		return fmt.Errorf("hub: save snapshot: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		slog.Warn("Saved a snapshot for a room that has no row", "room", roomID)
	}
	return nil
}
func (d dbStore) Preserve(ctx context.Context, roomID ulid.ULID, snapshot []byte) error {
	if err := d.q.KeepFailedSnapshot(ctx, queries.KeepFailedSnapshotParams{
		SnapshotFailed: snapshot,
		ID:             roomID,
	}); err != nil {
		return fmt.Errorf("hub: keep failed snapshot: %w", err)
	}
	return nil
}
func (d dbStore) AutosaveScene(ctx context.Context, roomID ulid.ULID, body []byte, preview *ulid.ULID) error {
	row, err := d.q.GetRoom(ctx, roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("hub: read the room's open scene: %w", err)
	}
	if row.SceneID == nil {
		return nil
	}
	scene, err := d.q.GetScene(ctx, queries.GetSceneParams{ID: *row.SceneID, OwnerID: row.OwnerID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("hub: read a scene before autosaving it: %w", err)
	}
	if !scene.Autosave {
		return nil
	}
	if _, err := d.q.UpdateSceneBody(ctx, queries.UpdateSceneBodyParams{
		Body:      body,
		PreviewID: preview,
		ID:        scene.ID,
		OwnerID:   row.OwnerID,
	}); err != nil {
		return fmt.Errorf("hub: autosave a scene: %w", err)
	}
	return nil
}
func (d dbStore) ClearMembership(ctx context.Context, roomID, userID ulid.ULID) error {
	_, err := d.q.ClearUserRoomSessions(ctx, queries.ClearUserRoomSessionsParams{
		RoomID: &roomID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("hub: clear membership: %w", err)
	}
	return nil
}
func sheetVitals(q *queries.Queries) func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
	return func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		_, err := q.UpdateCharacterFromPawn(ctx, queries.UpdateCharacterFromPawnParams{
			CurrentHP: column(v.HP),
			MaxHP:     max(1, column(v.MaxHP)),
			AC:        column(v.AC),
			Size:      string(v.Size),
			ID:        character,
		})
		return err
	}
}
func column(v int) uint16 {
	return uint16(max(0, min(v, math.MaxUint16)))
}
func hydrate(roomID ulid.ULID, l Loaded) (*room.State, bool) {
	failed := false
	s, err := room.Unmarshal(l.Snapshot)
	switch {
	case err == nil:
	case errors.Is(err, room.ErrEmpty):
		s = room.NewState(roomID, l.Name, room.Env{})
	case errors.Is(err, room.ErrSchema):
		slog.Error("Starting a room fresh and keeping the snapshot: it is from a schema this build cannot read", "room", roomID, "error", err)
		s = room.NewState(roomID, l.Name, room.Env{})
		failed = true
	default:
		slog.Error("Starting a room fresh and keeping the snapshot: it could not be read", "room", roomID, "error", err)
		s = room.NewState(roomID, l.Name, room.Env{})
		failed = true
	}
	s.Room.ID = roomID
	s.Room.Name = l.Name
	s.Room.Locked = l.Locked
	for i := range s.Players {
		s.Players[i].Connected = false
	}
	s.Music.Playing = false
	s.Music.At = 0
	s.Music.Since = 0
	s.Normalize()
	return s, failed
}
