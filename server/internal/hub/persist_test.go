package hub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"tabletopper/internal/room"
)

func TestADirtyRoomSavesOnTheIntervalAndACleanOneNever(t *testing.T) {
	tb := newTabletop(t, Options{SnapshotInterval: 200 * time.Millisecond})
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
	time.Sleep(400 * time.Millisecond)
	if got := tb.store.saved(); got != 1 {
		t.Errorf("the room saved %d times, want one write for one change", got)
	}
}
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
func TestARestartRestoresTheStateTheSequenceAndNobodysConnection(t *testing.T) {
	before := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	before.Seq = 41
	before.Players = []room.Player{{ID: playerID, Name: "Ari", Role: room.RolePlayer, Connected: true}}
	blob, err := room.Marshal(before)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
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
	err = tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: playerID, Role: room.RolePlayer}, &room.PawnRemove{})
	var refusal *room.Error
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want a *room.Error", err)
	}
	if refusal.Code != room.CodeForbidden {
		t.Errorf("code = %q, want %q", refusal.Code, room.CodeForbidden)
	}
}
func TestNotifyDoesNothingForARoomThatIsNotRunning(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.Notify(roomID, &room.RoomSetLocked{Locked: true})
	if tb.live(roomID) {
		t.Error("Notify loaded a room; it is a mirror, not a command")
	}
}
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
	if allowed, _ := b.take(now.Add(20 * time.Millisecond)); !allowed {
		t.Error("the bucket did not refill")
	}
}
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
func TestShutdownTellsEveryRoomToSaveEvenWhenTheCallersContextIsAlreadyDone(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tb.Shutdown(ctx)
	eventually(t, "the room to save on shutdown", func() bool { return tb.store.saved() == 1 })
}
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
	eventually(t, "the second save", func() bool { return tb.store.saved() >= 2 })
}
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
	tb := newTabletop(t, Options{})
	tb.store.loaded = Loaded{Name: "The Sunless Citadel", Snapshot: json.RawMessage("{}")}
	tb.join(gmID, "Kyle", room.RoleGM)
	if got := tb.store.kept(); len(got) != 0 {
		t.Errorf("an empty snapshot was kept as a failure: %q", got)
	}
}
