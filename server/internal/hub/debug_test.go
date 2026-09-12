package hub

import (
	"testing"
	"time"

	"tabletopper/internal/room"
)

func TestTheDebugViewCountsConnectionsPerUser(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)
	tb.join(playerID, "Ari", room.RolePlayer)
	tb.join(playerID, "Ari", room.RolePlayer)
	view, ok := tb.Debug(tb.ctx(), roomID)
	if !ok {
		t.Fatal("a loaded room answered nothing")
	}
	if view.Conns != 3 {
		t.Errorf("Conns = %d, want 3", view.Conns)
	}
	if len(view.PerUser) != 2 {
		t.Fatalf("PerUser has %d rows, want one per user", len(view.PerUser))
	}
	for _, conn := range view.PerUser {
		want := 1
		if conn.User == playerID.String() {
			want = 2
		}
		if conn.Count != want {
			t.Errorf("%s holds %d connections, want %d", conn.User, conn.Count, want)
		}
	}
}
func TestTheDebugViewReportsAnUnloadedRoomAsAbsent(t *testing.T) {
	tb := newTabletop(t, Options{})
	if _, ok := tb.Debug(tb.ctx(), roomID); ok {
		t.Error("a room nobody has opened answered; the view must not load one just to report on it")
	}
}
func TestTheDebugViewShowsTheSequencesDriftingApart(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.send(gm, "1", &room.TableAddLayer{Name: "Cellar"})
	view, ok := tb.Debug(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room answered nothing")
	}
	if view.Changes == 0 {
		t.Error("a command that changed the table was not counted")
	}
	if view.SeqGM < view.SeqPlayer {
		t.Errorf("seqGM %d is behind seqPlayer %d; the GM sees at least what a player does", view.SeqGM, view.SeqPlayer)
	}
	if view.Layers < 2 {
		t.Errorf("Layers = %d, want the floor that was just added", view.Layers)
	}
}
func TestTheDebugViewShowsWhatIsWaitingOnTheCoalesceTimer(t *testing.T) {
	tb := newTabletop(t, Options{CoalesceInterval: time.Hour})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	tb.send(gm, "1", &room.PawnDrag{Anchor: testID(9), X: 1, Y: 1})
	view, ok := tb.Debug(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room answered nothing")
	}
	if len(view.Coalescing) != 1 {
		t.Fatalf("Coalescing has %d keys, want the drag that is still waiting", len(view.Coalescing))
	}
	tb.flush()
	after, _ := tb.Debug(tb.ctx(), roomID)
	if len(after.Coalescing) != 0 {
		t.Errorf("Coalescing still holds %v after the window closed", after.Coalescing)
	}
}
func TestTheDebugViewCarriesTheLimitsItsCountersAreMeasuredAgainst(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)
	view, ok := tb.Debug(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room answered nothing")
	}
	if view.Limits.ConnsPerRoom == 0 || view.Limits.ConnsPerUser == 0 {
		t.Error("the caps are zero, so the panel cannot say how close the room is to them")
	}
	if view.InboxCap != inboxSize {
		t.Errorf("InboxCap = %d, want %d", view.InboxCap, inboxSize)
	}
	if view.SoftLimit != snapshotSoftLimit {
		t.Errorf("SoftLimit = %d, want the ceiling the actor warns at", view.SoftLimit)
	}
	if view.Loaded != 1 {
		t.Errorf("Loaded = %d, want the one room this hub is holding", view.Loaded)
	}
}
func TestTheDebugViewReportsTheSnapshotSizeOnceItHasBeenWritten(t *testing.T) {
	tb := newTabletop(t, Options{SnapshotInterval: 100 * time.Millisecond})
	tb.join(gmID, "Kyle", room.RoleGM)
	before, _ := tb.Debug(tb.ctx(), roomID)
	if before.SnapshotBytes != 0 || !before.SavedAt.IsZero() {
		t.Error("a room reports a snapshot it has not encoded yet")
	}
	eventually(t, "the snapshot to be written", func() bool { return tb.store.saved() == 1 })
	eventually(t, "the size to be reported", func() bool {
		view, ok := tb.Debug(tb.ctx(), roomID)
		return ok && view.SnapshotBytes > 0 && !view.SavedAt.IsZero()
	})
}
