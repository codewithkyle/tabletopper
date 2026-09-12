package hub

import (
	"encoding/json"
	"sync"
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
	joined := only(t, gm, "players.upserted")
	second := only(t, player, "snapshot")
	if first[0].Seq >= second[0].Seq {
		t.Errorf("the second snapshot's seq is %d, want more than the first's %d", second[0].Seq, first[0].Seq)
	}
	if joined[0].Seq != second[0].Seq {
		t.Errorf("the arrival = seq %d and the snapshot that follows it = seq %d; the two audiences advanced by different amounts on one ToAll event", joined[0].Seq, second[0].Seq)
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
	spawned := only(t, gm, "pawns.upserted")
	only(t, player, "pawns.upserted")
	pawn := ulidField(t, onePawn(t, spawned[0]), "id")
	tb.send(gm, "2", &room.PawnSetVisible{IDs: []ulid.ULID{pawn}, Visible: false})
	only(t, gm, "pawns.upserted")
	removed := only(t, player, "pawns.removed")
	if got := oneID(t, removed[0]); got != pawn {
		t.Errorf("pawns.removed named %s, want %s", got, pawn)
	}
}
func TestARefusedCommandLeavesTheStateAsItWas(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	ari, rin := testID(30), testID(31)
	tb.seat(playerID, ari, "Ari")
	tb.seat(otherID, rin, "Rin")
	frames(t, gm)
	tb.send(gm, "1", &room.PawnSpawnCharacters{Pawns: []room.Pawn{
		{Name: "Ari", Size: room.SizeMedium, LayerID: layer, X: 64, Y: 64, CharacterID: &ari},
		{Name: "Rin", LayerID: layer, X: 128, Y: 64, CharacterID: &rin},
	}})
	only(t, gm, "error")
	view, ok := tb.Table(tb.ctx(), roomID)
	if !ok {
		t.Fatal("the room would not say what is on the table")
	}
	if got := view.Pawns[layer]; got != 0 {
		t.Fatalf("%d pawns are on the table; the first of the two was applied before the second was refused", got)
	}
}
func TestDragsCoalesceIntoOneFrameForEverybodyElse(t *testing.T) {
	tb := newTabletop(t, Options{CoalesceInterval: 10 * time.Millisecond})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 64, Y: 64, Visible: true,
		Pawn: &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
	})
	pawn := ulidField(t, onePawn(t, only(t, gm, "pawns.upserted")[0]), "id")
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
	only(t, player, "players.removed", "player.kicked")
	only(t, gm, "players.removed")
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
	only(t, gm, "players.upserted")
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 64, Y: 64, Visible: true,
		Pawn: &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
	})
	spawned := only(t, gm, "pawns.upserted")
	pawn := ulidField(t, onePawn(t, spawned[0]), "id")
	gmStart := spawned[0].Seq
	start := only(t, player, "pawns.upserted")[0].Seq
	tb.send(gm, "2", &room.PawnSpawn{
		Kind: room.PawnMonster, Layer: layer, X: 128, Y: 64, Visible: false,
		Pawn: &room.Pawn{Name: "Ambusher", Size: room.SizeSmall},
	})
	tb.send(gm, "3", &room.PawnMove{Anchor: pawn, X: 192, Y: 64})
	tb.send(gm, "p", &room.Ping{Layer: layer, X: 1, Y: 1})
	gmFrames := frames(t, gm)
	contiguous(t, "the GM", gmStart, gmFrames)
	if got := changeFrames(gmFrames); got != 2 {
		t.Errorf("the GM received %d frames for the two commands that changed the room, want one each", got)
	}
	playerFrames := frames(t, player)
	contiguous(t, "a player", start, playerFrames)
	if got := changeFrames(playerFrames); got != 1 {
		t.Errorf("a player received %d frames, want the one command they could see", got)
	}
}
func TestABulkCommandIsOneFramePerRole(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	player := tb.join(playerID, "Ari", room.RolePlayer)
	for n := range 100 {
		tb.send(gm, "s", &room.PawnSpawn{
			Kind: room.PawnMonster, Layer: layer, X: 64 * n, Y: 64, Visible: n%2 == 0,
			Pawn: &room.Pawn{Name: "Goblin", Size: room.SizeSmall},
		})
	}
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "clear", &room.TableClear{})
	for who, fs := range map[string][]frame{"the GM": frames(t, gm), "a player": frames(t, player)} {
		if got := changeFrames(fs); got != 1 {
			t.Errorf("%s received %d frames for one command: %v", who, got, types(fs))
		}
	}
}
func TestALeaveOverHTTPClosesThatPersonsSockets(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	if err := tb.Dispatch(tb.ctx(), roomID, room.Actor{}, &room.PlayerLeave{ID: playerID}); err != nil {
		t.Fatalf("the leave was refused: %v", err)
	}
	only(t, gm, "players.removed")
	select {
	case <-player.quit:
	default:
		t.Fatal("somebody who left over HTTP was left holding an open socket")
	}
	if player.reason != reasonLeft {
		t.Errorf("close reason = %q, want %q", player.reason, reasonLeft)
	}
}
func TestAnyCoalescerIsCoalescedByItsKey(t *testing.T) {
	tb := newTabletop(t, Options{CoalesceInterval: time.Hour})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	seen := &applied{}
	for _, cmd := range []*coalescing{
		{key: "left", name: "left-1", seen: seen},
		{key: "left", name: "left-2", seen: seen},
		{key: "right", name: "right-1", seen: seen},
		{key: "left", name: "left-3", seen: seen},
	} {
		tb.send(gm, "c", cmd)
	}
	if got := seen.all(); len(got) != 0 {
		t.Fatalf("%v were applied while the window was open; a coalescer waits for it to close", got)
	}
	tb.flush()
	if got := seen.all(); !equal(got, []string{"left-3", "right-1"}) {
		t.Fatalf("applied %v, want [left-3 right-1]: the last of each key and nothing before it", got)
	}
}

type applied struct {
	mu    sync.Mutex
	names []string
}

func (a *applied) add(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.names = append(a.names, name)
}
func (a *applied) all() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.names...)
}

type coalescing struct {
	key  string
	name string
	seen *applied
}

func (c *coalescing) CoalesceKey() string                         { return c.key }
func (c *coalescing) Authorize(s *room.State, a room.Actor) error { return nil }
func (c *coalescing) Apply(s *room.State, a room.Actor, env room.Env) ([]room.Signal, error) {
	c.seen.add(c.name)
	return nil, nil
}
func changeFrames(fs []frame) int {
	n := 0
	for _, f := range fs {
		if f.Type == "changes" {
			n++
		}
	}
	return n
}
func contiguous(t *testing.T, who string, start uint64, fs []frame) {
	t.Helper()
	seq := start
	for _, f := range fs {
		if f.Type != "changes" {
			if f.Seq != seq {
				t.Errorf("%s: %s carries seq %d, want the current %d -- a transient frame must not advance the sequence", who, f.Type, f.Seq, seq)
			}
			continue
		}
		seq++
		if f.Seq != seq {
			t.Errorf("%s: a frame carries seq %d, want %d", who, f.Seq, seq)
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
