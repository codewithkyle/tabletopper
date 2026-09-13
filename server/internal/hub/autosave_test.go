package hub

import (
	"context"
	"testing"
	"time"

	"tabletopper/internal/room"
)

func markTheTable(t *testing.T, tb *tabletop) {
	t.Helper()
	err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.FogAdd{
		Layer:  tb.actor().state.Table.ActiveLayer,
		Kind:   room.ShapeRect,
		Mode:   room.FogHide,
		Points: []int{0, 0, 64, 64},
	})
	if err != nil {
		t.Fatalf("could not put a shape on the floor: %v", err)
	}
}
func autosavedWithin(t *testing.T, tb *tabletop, want int) [][]byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		found := tb.store.autosaved()
		if len(found) >= want {
			return found
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d scene write-backs, want %d", len(found), want)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestTheLastGMToLeaveOffersTheTableToTheOpenScene(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Strahd", room.RoleGM)
	player := tb.join(playerID, "Ireena", room.RolePlayer)
	markTheTable(t, tb)
	tb.leave(player)
	if found := tb.store.autosaved(); len(found) != 0 {
		t.Fatalf("a player leaving wrote the scene back %d times", len(found))
	}
	tb.leave(gm)
	body := autosavedWithin(t, tb, 1)[0]
	written, err := room.Unmarshal(body)
	if err != nil {
		t.Fatalf("the body written back does not decode: %v", err)
	}
	if len(written.Fog) != 1 {
		t.Errorf("the body carries %d fog shapes, want the one on the table", len(written.Fog))
	}
	if len(written.Players) != 0 {
		t.Errorf("the body carries %d players; a scene holds nobody", len(written.Players))
	}
}
func TestASecondGMConnectionHoldsTheSceneOpen(t *testing.T) {
	tb := newTabletop(t, Options{})
	one := tb.join(gmID, "Strahd", room.RoleGM)
	two := tb.join(otherID, "Rahadin", room.RoleGM)
	tb.leave(one)
	if found := tb.store.autosaved(); len(found) != 0 {
		t.Fatalf("the scene was written back with a GM still at the table (%d times)", len(found))
	}
	tb.leave(two)
	autosavedWithin(t, tb, 1)
}
func TestAGMWithTwoTabsOnlyWritesBackWhenTheLastOneGoes(t *testing.T) {
	tb := newTabletop(t, Options{})
	one := tb.join(gmID, "Strahd", room.RoleGM)
	two := tb.join(gmID, "Strahd", room.RoleGM)
	tb.leave(one)
	if found := tb.store.autosaved(); len(found) != 0 {
		t.Fatalf("closing one tab wrote the scene back %d times", len(found))
	}
	tb.leave(two)
	autosavedWithin(t, tb, 1)
}
func TestShuttingDownWritesTheSceneBackBesideTheSnapshot(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Strahd", room.RoleGM)
	markTheTable(t, tb)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tb.Shutdown(ctx)
	if saves := tb.store.saved(); saves == 0 {
		t.Error("the room snapshot was not written on the way out")
	}
	found := tb.store.autosaved()
	if len(found) != 1 {
		t.Fatalf("%d scene write-backs on the way out, want 1", len(found))
	}
	written, err := room.Unmarshal(found[0])
	if err != nil {
		t.Fatalf("the body written back does not decode: %v", err)
	}
	if len(written.Fog) != 1 {
		t.Errorf("the body carries %d fog shapes, want the one that was on the table", len(written.Fog))
	}
}
