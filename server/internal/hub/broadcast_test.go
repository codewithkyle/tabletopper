package hub

import (
	"testing"

	"tabletopper/internal/room"
)

type untidy struct{}

func (untidy) Authorize(s *room.State, a room.Actor) error { return nil }
func (untidy) Apply(s *room.State, a room.Actor, env room.Env) ([]room.Signal, error) {
	s.Initiative.Entries = nil
	s.Table.Palette = nil
	s.Fog = nil
	return nil, nil
}
func (tb *tabletop) changeCount() (uint64, bool) {
	tb.t.Helper()
	reply := make(chan any, 1)
	err := tb.actor().post(tb.ctx(), ask{fn: func(a *actor) any {
		return [2]any{a.changes, a.dirty}
	}, reply: reply})
	if err != nil {
		tb.t.Fatalf("asking the room: %v", err)
	}
	got := (<-reply).([2]any)
	return got[0].(uint64), got[1].(bool)
}
func (tb *tabletop) saved() {
	tb.t.Helper()
	reply := make(chan any, 1)
	if err := tb.actor().post(tb.ctx(), ask{fn: func(a *actor) any { a.dirty = false; return nil }, reply: reply}); err != nil {
		tb.t.Fatalf("asking the room: %v", err)
	}
	<-reply
}
func TestACommandThatChangesNothingSendsNothingAndSavesNothing(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.saved()
	was, _ := tb.changeCount()
	tb.send(gm, "1", untidy{})
	if got := frames(t, gm); len(got) != 0 {
		t.Errorf("the GM was sent %v for a command that changed nothing", types(got))
	}
	if got := frames(t, player); len(got) != 0 {
		t.Errorf("the player was sent %v for a command that changed nothing", types(got))
	}
	count, dirty := tb.changeCount()
	if count != was || dirty {
		t.Errorf("the change count went from %d to %d and dirty is %v; nothing changed, so nothing is to be saved", was, count, dirty)
	}
}
func TestAChangeOnlyTheGMCanSeeIsOneGMFrameAndNothingForThePlayers(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.saved()
	tb.send(gm, "1", &room.PawnSpawn{
		Kind:    room.PawnMonster,
		Layer:   layer,
		X:       64,
		Y:       64,
		Visible: false,
		Pawn:    &room.Pawn{Name: "Ambusher", Size: room.SizeMedium},
	})
	only(t, gm, "pawns.upserted")
	if got := frames(t, player); len(got) != 0 {
		t.Errorf("the player was sent %v for a pawn hidden from them", types(got))
	}
	if _, dirty := tb.changeCount(); !dirty {
		t.Error("a hidden pawn arrived and the room is not marked for saving")
	}
}
