package hub

import (
	"encoding/json"
	"testing"
	"time"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func TestJoiningSendsTheSnapshotLastAndTheArrivalFirst(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	first := only(t, gm, "snapshot")
	player := tb.join(playerID, "Ari", room.RolePlayer)
	joined := only(t, gm, "player.joined")
	second := only(t, player, "snapshot")
	if first[0].Seq >= second[0].Seq {
		t.Errorf("the second snapshot's seq is %d, want more than the first's %d", second[0].Seq, first[0].Seq)
	}
	if joined[0].Seq != second[0].Seq {
		t.Errorf("player.joined = seq %d and the snapshot that follows it = seq %d; the two audiences advanced by different amounts on one ToAll event", joined[0].Seq, second[0].Seq)
	}
	if got := stateSeq(t, second[0]); got != second[0].Seq {
		t.Errorf("snapshot state.seq = %d, envelope seq = %d", got, second[0].Seq)
	}
	if got := snapshotVersion(t, second[0]); got != "test-build" {
		t.Errorf("snapshot version = %q, want the hub's build", got)
	}
}
func TestARefusalGoesToTheSenderAloneWithItsCorrelationID(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(player, "a9", &room.TableSetGrid{Grid: room.Grid{
		Lines: room.GridLinesSolid, CellSize: 64, Color: "#000000FF", Snap: room.SnapCells, FeetPerCell: 5, Diagonals: room.DiagonalsEqual,
	}})
	got := only(t, player, "error")
	if got[0].CID != "a9" {
		t.Errorf("cid = %q, want %q", got[0].CID, "a9")
	}
	if code, _ := got[0].Body["code"].(string); code != room.CodeForbidden {
		t.Errorf("code = %q, want %q", code, room.CodeForbidden)
	}
	if rest := frames(t, gm); len(rest) != 0 {
		t.Errorf("the GM received %v; a refusal is the sender's business alone", types(rest))
	}
}
func TestHidingAPawnUpdatesTheGMAndRemovesItForPlayers(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind:    room.PawnMonster,
		Layer:   layer,
		X:       64,
		Y:       64,
		Visible: true,
		Pawn:    &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
	})
	spawned := only(t, gm, "pawn.spawned")
	only(t, player, "pawn.spawned")
	pawn := ulidField(t, spawned[0].Body["pawn"].(map[string]any), "id")
	tb.send(gm, "2", &room.PawnSetVisible{IDs: []ulid.ULID{pawn}, Visible: false})
	only(t, gm, "pawn.updated")
	removed := only(t, player, "pawn.removed")
	if got := ulidField(t, removed[0].Body, "id"); got != pawn {
		t.Errorf("pawn.removed named %s, want %s", got, pawn)
	}
}
func TestDragsCoalesceIntoOneFrameForEverybodyElse(t *testing.T) {
	tb := newTabletop(t, Options{DragInterval: 10 * time.Millisecond})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 64, Y: 64, Visible: true,
		Pawn: &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
	})
	pawn := ulidField(t, only(t, gm, "pawn.spawned")[0].Body["pawn"].(map[string]any), "id")
	frames(t, player)
	for _, x := range []int{100, 200, 300} {
		tb.send(gm, "d", &room.PawnDrag{Anchor: pawn, X: x, Y: 64})
	}
	eventually(t, "the drag flush", func() bool { return len(player.out) > 0 })
	tb.settle()
	got := only(t, player, "pawn.dragging")
	at := got[0].Body["pawns"].([]any)[0].(map[string]any)
	if x := at["x"].(float64); x != 300 {
		t.Errorf("the preview landed at x=%v, want the last position of the three", x)
	}
	if rest := frames(t, gm); len(rest) != 0 {
		t.Errorf("the dragging client received %v, want nothing", types(rest))
	}
}
func TestAClientThatCannotKeepUpIsDroppedAndTheRoomCarriesOn(t *testing.T) {
	tb := newTabletop(t, Options{SendBuffer: 1})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	select {
	case <-gm.quit:
	default:
		t.Fatal("the GM's connection was left open with a full buffer")
	}
	if gm.reason != reasonSlow {
		t.Errorf("close reason = %q, want %q", gm.reason, reasonSlow)
	}
	if gm.code != closePolicy {
		t.Errorf("close code = %v, want a policy violation", gm.code)
	}
	layer := activeLayer(t, only(t, player, "snapshot")[0])
	tb.send(player, "p", &room.Ping{Layer: layer, X: 10, Y: 10})
	only(t, player, "pinged")
}
func TestAKickTellsThePersonClosesThemAndForgetsTheirMembership(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "k", &room.PlayerKick{ID: playerID})
	only(t, player, "player.kicked")
	only(t, gm, "player.left")
	select {
	case <-player.quit:
	default:
		t.Fatal("the kicked player's connection was left open")
	}
	if player.reason != reasonKicked {
		t.Errorf("close reason = %q, want %q", player.reason, reasonKicked)
	}
	eventually(t, "the membership to be cleared", func() bool { return len(tb.store.clears()) == 1 })
	if got := tb.store.clears()[0]; got != [2]ulid.ULID{roomID, playerID} {
		t.Errorf("cleared %v, want the room and the kicked user", got)
	}
}
func TestEachAudienceSeesItsOwnSequenceWithNoGaps(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	only(t, player, "snapshot")
	only(t, gm, "player.joined")
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 64, Y: 64, Visible: true,
		Pawn: &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
	})
	spawned := only(t, gm, "pawn.spawned")
	pawn := ulidField(t, spawned[0].Body["pawn"].(map[string]any), "id")
	gmStart := spawned[0].Seq
	start := only(t, player, "pawn.spawned")[0].Seq
	tb.send(gm, "2", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 128, Y: 64, Visible: false,
		Pawn: &room.Pawn{Name: "Ambusher", Size: room.SizeSmall},
	})
	tb.send(gm, "3", &room.PawnMove{Anchor: pawn, X: 192, Y: 64})
	tb.send(gm, "p", &room.Ping{Layer: layer, X: 1, Y: 1})
	contiguous(t, "the GM", gmStart, frames(t, gm))
	contiguous(t, "a player", start, frames(t, player))
}
func contiguous(t *testing.T, who string, start uint64, fs []frame) {
	t.Helper()
	transient := map[string]bool{"error": true, "pawn.dragging": true, "pinged": true, "player.kicked": true, "room.closed": true}
	seq := start
	for _, f := range fs {
		if transient[f.Type] {
			if f.Seq != seq {
				t.Errorf("%s: %s carries seq %d, want the current %d -- a transient event must not advance the sequence", who, f.Type, f.Seq, seq)
			}
			continue
		}
		seq++
		if f.Seq != seq {
			t.Errorf("%s: %s carries seq %d, want %d", who, f.Type, f.Seq, seq)
			seq = f.Seq
		}
	}
}
func activeLayer(t *testing.T, f frame) ulid.ULID {
	t.Helper()
	state, ok := f.Body["state"].(map[string]any)
	if !ok {
		t.Fatalf("frame %q has no state", f.Type)
	}
	table, ok := state["table"].(map[string]any)
	if !ok {
		t.Fatal("the snapshot has no table")
	}
	return ulidField(t, table, "activeLayer")
}
func stateSeq(t *testing.T, f frame) uint64 {
	t.Helper()
	state, ok := f.Body["state"].(map[string]any)
	if !ok {
		t.Fatalf("frame %q has no state", f.Type)
	}
	return uint64(state["seq"].(float64))
}
func snapshotVersion(t *testing.T, f frame) string {
	t.Helper()
	v, _ := f.Body["version"].(string)
	return v
}
func ulidField(t *testing.T, body map[string]any, name string) ulid.ULID {
	t.Helper()
	raw, ok := body[name].(string)
	if !ok {
		encoded, _ := json.Marshal(body)
		t.Fatalf("field %q is not a string in %s", name, encoded)
	}
	id, err := ulid.Parse(raw)
	if err != nil {
		t.Fatalf("field %q is not a ULID: %v", name, err)
	}
	return id
}
