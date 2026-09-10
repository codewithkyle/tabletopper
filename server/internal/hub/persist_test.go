package hub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"tabletopper/internal/room"
)

// A room that changed is written back, and one that did not is not. The
// debounce is the whole design: a busy table writes one row every few seconds
// however many pawns moved, and an idle one writes nothing at all.
func TestADirtyRoomSavesOnTheIntervalAndACleanOneNever(t *testing.T) {
	tb := newTabletop(t, Options{SnapshotInterval: 200 * time.Millisecond})

	// Loading a room is not a change to it. Nobody has joined, so there is
	// nothing in memory the row does not already say.
	tb.actor()
	time.Sleep(400 * time.Millisecond)
	if got := tb.store.saved(); got != 0 {
		t.Fatalf("an untouched room saved %d times, want none", got)
	}

	tb.join(gmID, "Kyle", room.RoleGM)
	if got := tb.store.saved(); got != 0 {
		t.Errorf("the room saved %d times immediately, want the write to wait for the interval", got)
	}

	eventually(t, "the snapshot to be written", func() bool { return tb.store.saved() == 1 })

	// And it does not keep writing what it already wrote.
	time.Sleep(400 * time.Millisecond)
	if got := tb.store.saved(); got != 1 {
		t.Errorf("the room saved %d times, want one write for one change", got)
	}
}

// A deploy is a reconnect and not a lost session, which takes two things: the
// state is written before the process goes, and the sockets are closed with
// going-away so the browsers come back rather than showing an error.
func TestShutdownSavesEveryRoomAndSendsEverybodyAway(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tb.Shutdown(ctx)

	if got := tb.store.saved(); got != 1 {
		t.Errorf("shutdown saved %d times, want exactly one", got)
	}
	select {
	case <-gm.quit:
	default:
		t.Fatal("shutdown left a connection open")
	}
	if gm.code != closeGoingAway {
		t.Errorf("close code = %v, want going away so the client reconnects", gm.code)
	}
	if tb.live(roomID) {
		t.Error("the room is still live after shutdown")
	}
}

// An empty room stays loaded for a grace period, because a page reload is a
// close and an open a few hundred milliseconds apart and rehydrating in between
// would put a database read behind every F5.
func TestAnEmptyRoomUnloadsAfterTheGraceAndSavesOnTheWayOut(t *testing.T) {
	tb := newTabletop(t, Options{
		SnapshotInterval: 20 * time.Millisecond,
		UnloadGrace:      40 * time.Millisecond,
	})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.leave(gm)

	eventually(t, "the room to unload", func() bool { return !tb.live(roomID) })
	if got := tb.store.saved(); got == 0 {
		t.Error("the room unloaded without saving")
	}
}

// The grace exists to be interrupted. Somebody coming back inside it finds the
// room they left, with their pawns where they were.
func TestSomebodyComingBackInsideTheGraceKeepsTheRoomLoaded(t *testing.T) {
	tb := newTabletop(t, Options{
		SnapshotInterval: 20 * time.Millisecond,
		UnloadGrace:      time.Second,
	})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.leave(gm)
	time.Sleep(100 * time.Millisecond)
	tb.join(gmID, "Kyle", room.RoleGM)

	time.Sleep(200 * time.Millisecond)
	if !tb.live(roomID) {
		t.Error("the room unloaded while somebody was in it")
	}
}

// The three ways a snapshot can fail to decode all mean the same thing to the
// caller -- start fresh -- and they are told apart only by what gets logged.
func TestAnUnreadableSnapshotStartsTheRoomFreshFromTheRow(t *testing.T) {
	for _, c := range []struct{ name, snapshot string }{
		{"the column default, which is a room nobody has opened", "{}"},
		{"a snapshot from another schema", `{"schema":99,"seq":7,"room":{"id":"","name":"old","locked":false}}`},
		{"bytes that are not JSON at all", "not json"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, _ := hydrate(roomID, Loaded{Name: "The Sunless Citadel", Snapshot: json.RawMessage(c.snapshot)})

			if s.Room.Name != "The Sunless Citadel" {
				t.Errorf("name = %q, want the row's", s.Room.Name)
			}
			if s.Room.ID != roomID {
				t.Errorf("id = %s, want %s", s.Room.ID, roomID)
			}
			if s.Seq != 0 {
				t.Errorf("seq = %d, want a fresh room to start at zero", s.Seq)
			}
			if len(s.Table.Layers) != 1 {
				t.Errorf("layers = %d, want the one a new room is created with", len(s.Table.Layers))
			}
		})
	}
}

// The round trip a restart makes: what the room saved is what the next process
// picks up, including the sequence, and nobody comes back connected.
func TestARestartRestoresTheStateTheSequenceAndNobodysConnection(t *testing.T) {
	before := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	before.Seq = 41
	before.Players = []room.Player{{ID: playerID, Name: "Ari", Role: room.RolePlayer, Connected: true}}

	blob, err := room.Marshal(before)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The row is the writer of record for the name and the lock, and a GM can
	// change either while the room is not even loaded -- so the snapshot's copy
	// of them is a souvenir and the row wins.
	after, _ := hydrate(roomID, Loaded{Name: "The Forge of Fury", Locked: true, Snapshot: blob})

	if after.Seq != 41 {
		t.Errorf("seq = %d, want the saved 41", after.Seq)
	}
	if after.Room.Name != "The Forge of Fury" || !after.Room.Locked {
		t.Errorf("room = %+v, want the row's name and lock", after.Room)
	}
	if len(after.Players) != 1 {
		t.Fatalf("players = %d, want the one that was saved", len(after.Players))
	}
	if after.Players[0].Connected {
		t.Error("a player came back connected; the process they were connected to is gone")
	}
}

// A room that is not live still has to answer a control on the room page. The
// GM clicked something and expects it to have happened, so Dispatch loads
// rather than doing nothing -- which is the whole difference from Notify.
func TestDispatchLoadsARoomThatIsNotRunning(t *testing.T) {
	tb := newTabletop(t, Options{})

	if tb.live(roomID) {
		t.Fatal("the room was live before anything asked for it")
	}

	err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.RoomSetName{Name: "The Forge of Fury"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !tb.live(roomID) {
		t.Error("the room did not load")
	}

	// And a refusal comes back as the protocol's own error, which is what lets
	// a handler put a heading and a message in the alert modal without knowing
	// anything about the command it sent.
	err = tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: playerID, Role: room.RolePlayer}, &room.PawnRemove{})
	var refusal *room.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want a *room.Error", err)
	}
	if refusal.Code != room.CodeForbidden {
		t.Errorf("code = %q, want %q", refusal.Code, room.CodeForbidden)
	}
}

// Notify is the other half of that pair and does nothing when the room is not
// running, because the rooms row is the writer of record for everything it
// carries and the next load reads it.
func TestNotifyDoesNothingForARoomThatIsNotRunning(t *testing.T) {
	tb := newTabletop(t, Options{})

	tb.Notify(roomID, &room.RoomSetLocked{Locked: true})

	if tb.live(roomID) {
		t.Error("Notify loaded a room; it is a mirror, not a command")
	}
}

// The per-connection rate limit: a bucket that refills, a refusal when it is
// empty, and the door after enough refusals inside the window.
func TestTheRateLimitRefusesAtTheBoundaryAndClosesAfterRepeatedOvers(t *testing.T) {
	opts := Options{Rate: 100, Burst: 3, Overs: 3, OverWindow: time.Minute}.withDefaults()
	b := newBucket(opts)

	now := time.Now()
	b.last, b.tokens = now, float64(opts.Burst)

	for i := range opts.Burst {
		if allowed, _ := b.take(now); !allowed {
			t.Fatalf("frame %d was refused inside the burst", i+1)
		}
	}

	for i := range opts.Overs {
		allowed, over := b.take(now)
		if allowed {
			t.Fatalf("frame %d was allowed past the burst", i+1)
		}
		if want := i+1 == opts.Overs; over != want {
			t.Errorf("over after %d refusals = %v, want %v", i+1, over, want)
		}
	}

	// And it refills: a hundred a second is a token every ten milliseconds.
	if allowed, _ := b.take(now.Add(20 * time.Millisecond)); !allowed {
		t.Error("the bucket did not refill")
	}
}

// Closing is the end of a room in memory as well as in the row. Everybody is
// told, everybody is dropped, the state is written one last time so reopening
// comes back to the pawns where they were left, and the room stops being live
// -- which is what lets a reopen load a fresh one rather than find a goroutine
// that has already gone.
func TestClosingARoomTellsEverybodySavesAndUnloadsIt(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)

	tb.Close(tb.ctx(), roomID)

	only(t, gm, "room.closed")
	only(t, player, "room.closed")

	if tb.live(roomID) {
		t.Error("the room is still live after being closed")
	}
	if got := tb.store.saved(); got != 1 {
		t.Errorf("closing saved %d times, want exactly one -- reopening has to come back to the pawns", got)
	}
}

// A room can be loaded by something that never connects: a GM toggling the lock
// from a page whose socket has not opened yet. It has to unload on the same
// grace as any other, or the process accumulates one room per settings change
// for as long as it runs.
func TestARoomNobodyConnectedToStillUnloads(t *testing.T) {
	tb := newTabletop(t, Options{
		SnapshotInterval: 20 * time.Millisecond,
		UnloadGrace:      40 * time.Millisecond,
	})

	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.RoomSetLocked{Locked: true}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !tb.live(roomID) {
		t.Fatal("the room did not load")
	}

	eventually(t, "the room to unload", func() bool { return !tb.live(roomID) })
}

// The path the whole snapshot design exists for, on its worst day: the caller's
// context is already done. The rooms are told to save regardless, because the
// telling does not depend on that context -- only the wait does.
func TestShutdownTellsEveryRoomToSaveEvenWhenTheCallersContextIsAlreadyDone(t *testing.T) {
	tb := newTabletop(t, Options{})

	tb.join(gmID, "Kyle", room.RoleGM)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tb.Shutdown(ctx)

	eventually(t, "the room to save on shutdown", func() bool { return tb.store.saved() == 1 })
}

// The snapshot is written off the room's goroutine. A database that takes a
// second to answer used to be a second every command at the table waited,
// every five seconds, for the whole of a fight.
func TestASlowSaveDoesNotStallTheRoom(t *testing.T) {
	tb := newTabletop(t, Options{SnapshotInterval: 20 * time.Millisecond})
	tb.store.saveDelay = 500 * time.Millisecond

	tb.join(gmID, "Kyle", room.RoleGM)
	eventually(t, "a save to be in flight", func() bool { return tb.store.saving() > 0 })

	started := time.Now()
	err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.RoomSetName{Name: "The Forge of Fury"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if took := time.Since(started); took > 200*time.Millisecond {
		t.Errorf("a command took %v while a save was in flight; the room was waiting on the database", took)
	}

	// And the change that landed during the write is not lost: the room stays
	// dirty and the next tick writes it.
	eventually(t, "the second save", func() bool { return tb.store.saved() >= 2 })
}

// A room whose row has been refusing its snapshot stays loaded, empty or not,
// until the database takes it. Unloading would throw the table away, and an
// outage that ends an hour later would find nothing to come back to.
func TestARoomWithAWriteOwedStaysLoadedUntilTheDatabaseTakesIt(t *testing.T) {
	tb := newTabletop(t, Options{
		SnapshotInterval: 20 * time.Millisecond,
		UnloadGrace:      40 * time.Millisecond,
	})
	tb.store.failSaves(errors.New("the database is away"))

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.leave(gm)

	time.Sleep(200 * time.Millisecond)
	if !tb.live(roomID) {
		t.Fatal("the room unloaded with a snapshot it had never managed to write")
	}

	tb.store.failSaves(nil)
	eventually(t, "the room to save and unload", func() bool { return tb.store.saved() > 0 && !tb.live(roomID) })
}

// A SNAPSHOT THAT CANNOT BE READ IS KEPT, not overwritten. The fresh room's
// first save used to land on the column the bad blob came from, and the only
// copy of the old table was gone the moment anybody moved a pawn.
func TestAnUnreadableSnapshotIsKeptBeforeTheFreshRoomSavesOverIt(t *testing.T) {
	for _, c := range []struct{ name, snapshot string }{
		{"a snapshot from a newer schema", `{"schema":99,"seq":7,"room":{"id":"","name":"later","locked":false}}`},
		{"bytes that are not JSON", "not json"},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := newTabletop(t, Options{SnapshotInterval: 20 * time.Millisecond})
			tb.store.loaded = Loaded{Name: "The Sunless Citadel", Snapshot: json.RawMessage(c.snapshot)}

			tb.join(gmID, "Kyle", room.RoleGM)

			eventually(t, "the bad snapshot to be kept", func() bool { return len(tb.store.kept()) == 1 })
			if got := string(tb.store.kept()[0]); got != c.snapshot {
				t.Errorf("kept %q, want the bytes as they were", got)
			}
		})
	}

	// And an ordinary first join keeps nothing: the empty object is the column
	// default and not a failure.
	tb := newTabletop(t, Options{})
	tb.store.loaded = Loaded{Name: "The Sunless Citadel", Snapshot: json.RawMessage("{}")}
	tb.join(gmID, "Kyle", room.RoleGM)
	if got := tb.store.kept(); len(got) != 0 {
		t.Errorf("an empty snapshot was kept as a failure: %q", got)
	}
}
