package hub

import (
	"testing"

	"tabletopper/internal/room"
)

type tableSwap struct {
	room.Resync
}

func (c *tableSwap) Authorize(s *room.State, a room.Actor) error { return nil }
func (c *tableSwap) Apply(s *room.State, a room.Actor, env room.Env) ([]room.Signal, error) {
	s.Table.Grid.CellSize = 96
	s.Pawns = append(s.Pawns, room.Pawn{
		ID:      testID(900),
		Kind:    room.PawnMonster,
		LayerID: s.Table.ActiveLayer,
		Name:    "Ambusher",
		Size:    room.SizeMedium,
		Visible: false,
	})
	s.Normalize()
	return nil, nil
}
func lastSeq(t *testing.T, c *client) uint64 {
	t.Helper()
	seen := frames(t, c)
	if len(seen) == 0 {
		t.Fatal("the client was sent nothing at all before the resync")
	}
	return seen[len(seen)-1].Seq
}
func pawnNames(t *testing.T, f frame) []string {
	t.Helper()
	state, ok := f.Body["state"].(map[string]any)
	if !ok {
		t.Fatalf("frame %q has no state", f.Type)
	}
	pawns, ok := state["pawns"].([]any)
	if !ok {
		t.Fatalf("the snapshot has no pawns: %v", state["pawns"])
	}
	out := make([]string, 0, len(pawns))
	for _, one := range pawns {
		body, ok := one.(map[string]any)
		if !ok {
			t.Fatalf("a pawn is not an object: %v", one)
		}
		name, _ := body["name"].(string)
		out = append(out, name)
	}
	return out
}
func roleOf(t *testing.T, f frame) string {
	t.Helper()
	you, ok := f.Body["you"].(map[string]any)
	if !ok {
		t.Fatalf("frame %q does not say who it is for", f.Type)
	}
	role, _ := you["role"].(string)
	return role
}
func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
func TestAResyncingCommandHandsEverybodyTheirOwnSnapshotAndNoDiff(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	pc := tb.join(playerID, "Ari", room.RolePlayer)
	gmSeq, pcSeq := lastSeq(t, gm), lastSeq(t, pc)

	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &tableSwap{}); err != nil {
		t.Fatalf("the swap was refused: %v", err)
	}

	gmFrame := only(t, gm, "snapshot")[0]
	pcFrame := only(t, pc, "snapshot")[0]
	if got := roleOf(t, gmFrame); got != string(room.RoleGM) {
		t.Errorf("the GM's snapshot says they are a %q", got)
	}
	if got := roleOf(t, pcFrame); got != string(room.RolePlayer) {
		t.Errorf("the player's snapshot says they are a %q", got)
	}
	if !contains(pawnNames(t, gmFrame), "Ambusher") {
		t.Errorf("the GM's snapshot hides the ambusher from them: %v", pawnNames(t, gmFrame))
	}
	if contains(pawnNames(t, pcFrame), "Ambusher") {
		t.Errorf("the player's snapshot shows them the ambusher: %v", pawnNames(t, pcFrame))
	}
	for name, pair := range map[string][2]uint64{
		"the GM":     {gmFrame.Seq, gmSeq},
		"the player": {pcFrame.Seq, pcSeq},
	} {
		if pair[0] != pair[1]+1 {
			t.Errorf("%s last saw seq %d and the snapshot is seq %d, want %d", name, pair[1], pair[0], pair[1]+1)
		}
	}
	if got := stateSeq(t, gmFrame); got != gmFrame.Seq {
		t.Errorf("the snapshot's frame is seq %d and its state is seq %d", gmFrame.Seq, got)
	}
}
func TestAResyncingCommandLeavesTheRoomToBeSaved(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &tableSwap{}); err != nil {
		t.Fatalf("the swap was refused: %v", err)
	}
	reply := make(chan any, 1)
	if err := tb.actor().post(tb.ctx(), ask{fn: func(a *actor) any { a.saveNow(); return nil }, reply: reply}); err != nil {
		t.Fatalf("the room would not save: %v", err)
	}
	<-reply
	if tb.store.saved() == 0 {
		t.Fatal("a resyncing command left the room clean, so the table it replaced comes back on the next restart")
	}
}
func TestACommandThatChangesNothingResyncsNobody(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	frames(t, gm)
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &idleSwap{}); err != nil {
		t.Fatalf("the swap was refused: %v", err)
	}
	only(t, gm)
}

type idleSwap struct {
	room.Resync
}

func (c *idleSwap) Authorize(s *room.State, a room.Actor) error { return nil }
func (c *idleSwap) Apply(s *room.State, a room.Actor, env room.Env) ([]room.Signal, error) {
	return nil, nil
}
