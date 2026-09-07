package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

func TestNextExpirySlidesForwardWhileYoung(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	now := created.Add(24 * time.Hour)

	if got, want := nextExpiry(now, created), now.Add(IdleWindow); !got.Equal(want) {
		t.Errorf("nextExpiry = %v, want %v", got, want)
	}
}

func TestNextExpiryClampsToMaxLifetime(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cap := created.Add(MaxLifetime)

	// a refresh landing just inside the cap must not extend a full idle window past it
	now := cap.Add(-time.Minute)

	got := nextExpiry(now, created)
	if !got.Equal(cap) {
		t.Errorf("nextExpiry = %v, want the cap %v", got, cap)
	}
	if got.After(cap) {
		t.Errorf("nextExpiry = %v exceeds the cap %v", got, cap)
	}
}

func TestNextExpiryNeverExceedsCapAcrossLifetime(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cap := created.Add(MaxLifetime)

	for d := time.Duration(0); d < MaxLifetime; d += 12 * time.Hour {
		if got := nextExpiry(created.Add(d), created); got.After(cap) {
			t.Fatalf("at +%s nextExpiry = %v, exceeds cap %v", d, got, cap)
		}
	}
}

// roomRecordingDB is a queries.DBTX that keeps the statements it was given, for
// the two room writes -- which are about what went out and not about what came
// back. Only ExecContext is reachable from either; the other three are here
// because DBTX declares them, and a call to one of them from this path would be
// a change worth failing on.
type roomRecordingDB struct {
	t    *testing.T
	sent []string
	args [][]any
	rows int64
}

func (d *roomRecordingDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	d.sent = append(d.sent, query)
	d.args = append(d.args, args)

	return roomResult{rows: d.rows}, nil
}

func (d *roomRecordingDB) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	d.t.Fatal("a room write prepared a statement")

	return nil, nil
}

func (d *roomRecordingDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	d.t.Fatal("a room write ran a query")

	return nil, nil
}

func (d *roomRecordingDB) QueryRowContext(context.Context, string, ...any) *sql.Row {
	d.t.Fatal("a room write ran a query")

	return nil
}

type roomResult struct{ rows int64 }

func (r roomResult) LastInsertId() (int64, error) { return 0, nil }
func (r roomResult) RowsAffected() (int64, error) { return r.rows, nil }

// THE STRUCT IS MUTATED ALONGSIDE THE ROW, and that is the half worth pinning.
// The handler renders from the copy the middleware took a moment earlier, so a
// join that wrote the row and left the copy alone would seat somebody at a
// table and then draw them the page of somebody who is not in one.
func TestJoinRoomWritesTheRowAndTheCopy(t *testing.T) {
	db := &roomRecordingDB{t: t, rows: 1}
	store := NewStore(queries.New(db), false)

	roomID := ulid.Make()
	characterID := ulid.Make()
	s := UserSession{Hash: []byte("hash")}

	if err := store.JoinRoom(context.Background(), &s, roomID, &characterID); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if len(db.sent) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.sent))
	}
	if !strings.Contains(db.sent[0], "UPDATE sessions") || !strings.Contains(db.sent[0], "room_id = ?") {
		t.Errorf("statement is not the room write: %q", db.sent[0])
	}

	// Keyed on the hash, like every other statement in this package: the hash
	// is what the store holds for the request in flight.
	args := db.args[0]
	if len(args) != 3 {
		t.Fatalf("statement took %d values, want 3", len(args))
	}
	if hash, ok := args[2].([]byte); !ok || string(hash) != "hash" {
		t.Errorf("the statement is keyed on %v, want the session hash", args[2])
	}

	if s.RoomID == nil || *s.RoomID != roomID {
		t.Errorf("RoomID = %v, want %v", s.RoomID, roomID)
	}
	if s.CharacterID == nil || *s.CharacterID != characterID {
		t.Errorf("CharacterID = %v, want %v", s.CharacterID, characterID)
	}
}

// Joining with no character is a join, not a refusal: a player may sit down
// before they have made one.
func TestJoinRoomAcceptsNoCharacter(t *testing.T) {
	db := &roomRecordingDB{t: t, rows: 1}
	store := NewStore(queries.New(db), false)

	s := UserSession{Hash: []byte("hash")}
	if err := store.JoinRoom(context.Background(), &s, ulid.Make(), nil); err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if s.CharacterID != nil {
		t.Errorf("CharacterID = %v, want nil", s.CharacterID)
	}
	if db.args[0][1] != (*ulid.ULID)(nil) {
		t.Errorf("character bound as %#v, want a nil pointer so the column is written NULL", db.args[0][1])
	}
}

func TestLeaveRoomClearsTheRowAndTheCopy(t *testing.T) {
	db := &roomRecordingDB{t: t, rows: 1}
	store := NewStore(queries.New(db), false)

	roomID := ulid.Make()
	characterID := ulid.Make()
	s := UserSession{Hash: []byte("hash"), RoomID: &roomID, CharacterID: &characterID}

	if err := store.LeaveRoom(context.Background(), &s); err != nil {
		t.Fatalf("LeaveRoom: %v", err)
	}

	if len(db.sent) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.sent))
	}
	if !strings.Contains(db.sent[0], "room_id = NULL") || !strings.Contains(db.sent[0], "character_id = NULL") {
		t.Errorf("statement does not clear both columns: %q", db.sent[0])
	}
	if s.RoomID != nil || s.CharacterID != nil {
		t.Errorf("the copy still says room %v, character %v", s.RoomID, s.CharacterID)
	}
}

// A statement that matched nothing means the session ended between the
// middleware reading it and the handler writing it. Reporting success there
// would tell somebody they had joined a room their browser can no longer prove
// it belongs to.
func TestARoomWriteThatMatchesNothingIsAnError(t *testing.T) {
	db := &roomRecordingDB{t: t, rows: 0}
	store := NewStore(queries.New(db), false)

	s := UserSession{Hash: []byte("hash")}
	if err := store.JoinRoom(context.Background(), &s, ulid.Make(), nil); !errors.Is(err, ErrSessionGone) {
		t.Errorf("JoinRoom error = %v, want ErrSessionGone", err)
	}
	if s.RoomID != nil {
		t.Error("the copy was mutated after a write that matched no row")
	}

	if err := store.LeaveRoom(context.Background(), &s); !errors.Is(err, ErrSessionGone) {
		t.Errorf("LeaveRoom error = %v, want ErrSessionGone", err)
	}
}
