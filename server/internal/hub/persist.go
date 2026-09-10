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

// ErrNoRoom is a room that is not there to load: deleted, closed, or an id off
// a stale page. It is the one Load error a caller acts on rather than logs --
// the socket handler answers it with a 404 and the page sends the browser back
// to the join form.
var ErrNoRoom = errors.New("hub: no such open room")

// Store is everything the hub does to the database, which is three statements.
// It is an interface for one reason: the actor's tests drive real rooms through
// real commands, and a room that needed MySQL to start would make those
// integration tests rather than unit tests.
//
// EVERY METHOD IS CALLED FROM THE ROOM'S OWN GOROUTINE except Load, which runs
// on whichever goroutine asked for a room that was not live. An implementation
// therefore has to be safe for concurrent use, which *queries.Queries over a
// *sql.DB already is.
type Store interface {
	// Load is the rooms row: the two columns the row is the writer of record
	// for, and the snapshot to rehydrate from. A room that does not exist or
	// is closed answers ErrNoRoom.
	Load(ctx context.Context, roomID ulid.ULID) (Loaded, error)

	// Save writes the debounced snapshot. seq goes in its own column for the
	// benefit of anybody reading the table by hand; the state carries its own
	// copy, and that is the one a load restores from.
	Save(ctx context.Context, roomID ulid.ULID, snapshot []byte, seq uint64) error

	// ClearMembership is what a kick owes the browser it removed: the session
	// rows stop pointing at the room, so the homepage stops offering to take
	// them back to it and a reload lands on the join page.
	ClearMembership(ctx context.Context, roomID, userID ulid.ULID) error

	// Preserve keeps a snapshot this build could not read, so that the fresh
	// room's first save does not overwrite the only copy of the old table.
	Preserve(ctx context.Context, roomID ulid.ULID, snapshot []byte) error
}

// Loaded is the rooms row, minus everything the hub has no use for.
type Loaded struct {
	Name     string
	Locked   bool
	Snapshot json.RawMessage
}

// NewStore is the production Store, over the generated queries.
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

	// A SAVE THAT MATCHED NO ROW IS NOT AN ERROR AND IS NOT SILENT EITHER. It
	// is a room saving into a row that was deleted, which DeleteRoom now ends
	// by closing the live room -- so this is the line that says so if some
	// other path ever deletes a row out from under a running room.
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

// sheetHP is the production WriteHP: one column on the characters row, and the
// clamp that keeps a number the protocol allows inside the column that holds
// it.
func sheetHP(q *queries.Queries) func(ctx context.Context, character ulid.ULID, hp int) error {
	return func(ctx context.Context, character ulid.ULID, hp int) error {
		_, err := q.UpdateCharacterCurrentHP(ctx, queries.UpdateCharacterCurrentHPParams{
			CurrentHP: uint16(max(0, min(hp, math.MaxUint16))),
			ID:        character,
		})

		return err
	}
}

// hydrate turns a loaded row into the state the room goroutine will own, and
// reports whether the snapshot it was given had to be thrown away.
//
// EVERY DECODE FAILURE STARTS THE ROOM FRESH, and the three are told apart by
// what gets logged and by the second answer. An empty snapshot is the ordinary
// first join to a room nobody has opened and is not worth a line; a snapshot
// from a schema this build cannot read, or one that is not JSON, means a live
// table just lost its pawns, which is worth a loud one -- and is worth keeping,
// which is what the caller does with a true.
//
// A SNAPSHOT FROM AN OLDER SCHEMA IS NOT A FAILURE. room.Unmarshal migrates it,
// so the room comes back with its table; what still fails is a snapshot from a
// NEWER schema, which is a rollback, and bytes that do not decode at all.
//
// THE ROW OVERWRITES THE SNAPSHOT for the name and the lock. Both are columns
// that HTTP routes write while the room is not even loaded -- a GM can rename a
// closed room from the rooms list -- so the snapshot's copy of them is a
// souvenir of the last time the room ran, and the row is the truth.
//
// EVERYBODY COMES BACK DISCONNECTED. The snapshot was written with people at
// the table and is being read because the process restarted, so a Connected
// that survived would show a player list full of people who are not there until
// each of them happened to reconnect.
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
	s.Normalize()

	return s, failed
}
