package room

import (
	"errors"
	"testing"

	"github.com/oklog/ulid/v2"
)

// A snapshot goes out and comes back the same. It is the same marshaller that
// writes the database column and encodes the message a connecting client
// receives, so this one round trip covers both.
func TestASnapshotRoundTrips(t *testing.T) {
	w := busyWorld(t)

	b, err := Marshal(w.s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	back, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	again, err := Marshal(back)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(again) != string(b) {
		t.Fatalf("the snapshot changed on the way through:\n%s\n%s", b, again)
	}
}

// THE COLUMN DEFAULT IS AN EMPTY OBJECT, so this is what the hub sees on the
// first join to a room nobody has opened. It is not a problem and it is not
// logged, which is why it is a distinct error from a schema break.
func TestAnEmptySnapshotIsItsOwnAnswer(t *testing.T) {
	for _, empty := range []string{"", "{}", "{ }", "\n{}\n"} {
		_, err := Unmarshal([]byte(empty))
		if !errors.Is(err, ErrEmpty) {
			t.Fatalf("Unmarshal(%q) = %v, want ErrEmpty", empty, err)
		}
	}
}

// A snapshot this build cannot read starts the room fresh, like an empty one --
// but it means a live table just lost its pawns to a deploy, so the hub can
// tell the two apart and log this one.
func TestASnapshotFromAnotherSchemaIsRefused(t *testing.T) {
	_, err := Unmarshal([]byte(`{"schema":99,"seq":4}`))
	if !errors.Is(err, ErrSchema) {
		t.Fatalf("Unmarshal of schema 99 = %v, want ErrSchema", err)
	}

	// And a snapshot with no schema at all is the same case: schema zero is
	// not this schema.
	_, err = Unmarshal([]byte(`{"seq":4}`))
	if !errors.Is(err, ErrSchema) {
		t.Fatalf("Unmarshal of a snapshot with no schema = %v, want ErrSchema", err)
	}
}

// BYTE STABILITY IS THE PROPERTY NORMALIZE EXISTS FOR. Two states holding the
// same facts marshal identically, whatever order they were built in, which is
// what lets every other test compare states by comparing strings.
func TestEqualStatesMarshalToEqualBytes(t *testing.T) {
	first := busyWorld(t)

	// The same room, built in a different order and with the collections
	// deliberately shuffled afterwards.
	second := busyWorld(t)
	second.s.Pawns = append(second.s.Pawns[1:], second.s.Pawns[0])
	second.s.Players = append(second.s.Players[1:], second.s.Players[0])
	second.s.Strokes = append(second.s.Strokes[1:], second.s.Strokes[0])
	second.s.Normalize()

	a, err := Marshal(first.s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b, err := Marshal(second.s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(a) != string(b) {
		t.Fatalf("two equal rooms marshalled differently:\n%s\n%s", a, b)
	}
}

// Project hands back something the caller may keep and edit. Sharing a backing
// array with the live room would make a snapshot event a way to corrupt it.
func TestAProjectionDoesNotShareStorageWithTheRoom(t *testing.T) {
	w := busyWorld(t)

	before := mustJSON(t, w.s)

	copyOfRoom := w.s.Project(RoleGM)
	copyOfRoom.Pawns[0].Name = "Edited"
	copyOfRoom.Pawns[0].X = 999_999
	copyOfRoom.Table.Layers[0].Name = "Edited"
	if len(copyOfRoom.Fog) > 0 {
		copyOfRoom.Fog[0].Points[0] = 999
	}
	if len(copyOfRoom.Strokes) > 0 {
		copyOfRoom.Strokes[0].Points[0] = 999
	}
	copyOfRoom.Initiative.Entries[0].Name = "Edited"

	if got := mustJSON(t, w.s); got != before {
		t.Fatal("editing a projection changed the room it came from")
	}
}

// The snapshot event is the first message after connect and the answer to a
// resync, and it is projected for whoever asked.
func TestSyncRequestAnswersTheAskerAlone(t *testing.T) {
	w := busyWorld(t)

	ems := w.apply(&SyncRequest{}, w.pc)
	equalStrings(t, "emissions", summary(ems), []string{"snapshot to sender"})

	snap := ems[0].Event.(*Snapshot)
	if snap.You.ID != testPlayerID || snap.You.Role != RolePlayer {
		t.Fatalf("the snapshot says the receiver is %+v", snap.You)
	}
	if snap.Version != "test-build" {
		t.Fatalf("the snapshot carries version %q", snap.Version)
	}
	if mustJSON(t, snap.State) != mustJSON(t, w.s.Project(RolePlayer)) {
		t.Fatal("the snapshot is not the player's projection")
	}

	// And the GM's is the whole room.
	gm := w.apply(&SyncRequest{}, w.gm)[0].Event.(*Snapshot)
	if mustJSON(t, gm.State) != mustJSON(t, w.s.Project(RoleGM)) {
		t.Fatal("the GM's snapshot is not the complete room")
	}
	if len(gm.State.Pawns) <= len(snap.State.Pawns) {
		t.Fatalf("the GM sees %d pawns and a player sees %d; the fixture hides nothing", len(gm.State.Pawns), len(snap.State.Pawns))
	}
}

// busyWorld is a room with something of everything in it, which is what the
// round-trip and stability tests need to be worth running.
func busyWorld(t *testing.T) *world {
	t.Helper()

	w := newWorld(t)
	cellar := w.addLayer("Cellar")

	w.apply(&TableSetLayerMap{
		Layer:   w.layer,
		AssetID: testAssetID,
		Map:     &MapRef{AssetID: testAssetID, Gen: testID(50), Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3},
	}, w.gm)

	ari := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", X: 96, Y: 96, Visible: true, OwnerID: &testPlayerID, CharacterID: &testCharID, HP: intp(11), MaxHP: intp(14), AC: intp(16)})
	goblin := w.spawn(Pawn{Kind: PawnMonster, Name: "Goblin", X: 160, Y: 96, Visible: true, HP: intp(3), MaxHP: intp(7), AC: intp(15), MonsterID: idp(testID(60))})
	w.spawn(Pawn{Kind: PawnMonster, Name: "Ambusher", X: 224, Y: 96, Visible: false, HP: intp(7), MaxHP: intp(7)})
	w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", Width: 128, Height: 256, X: 128, Y: 128, Visible: true})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Innkeeper", LayerID: cellar, Visible: true, HP: intp(4), MaxHP: intp(4)})

	w.apply(&PawnSetConditions{ID: goblin, Conditions: []Condition{
		{Name: "Prone", Color: ColorWhite, Duration: -1, Clear: ClearEnd},
		{Name: "Blessed", Color: ColorYellow, Duration: 3, Clear: ClearStart},
	}}, w.gm)

	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari", PawnIDs: []ulid.ULID{ari}, Initiative: 18},
		{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}, Initiative: 12},
		{Name: "Lair action", Initiative: 20},
	}}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 512, 512}}, w.gm)
	w.apply(&FogAdd{Layer: w.layer, Kind: ShapePoly, Mode: FogReveal, Points: []int{64, 64, 192, 64, 192, 192}}, w.gm)

	w.apply(&StrokeBegin{ID: testID(800), Layer: w.layer, Color: "#ff0000ff", Width: 4, Points: []int{0, 0, 8, 8}}, w.gm)
	w.apply(&StrokeExtend{ID: testID(800), Points: []int{16, 16}}, w.gm)
	w.apply(&StrokeEnd{ID: testID(800)}, w.gm)
	w.apply(&StrokeBegin{ID: testID(801), Layer: w.layer, Color: "#00ff00", Width: 2, Points: []int{4, 4}}, w.pc)
	w.apply(&StrokeBegin{ID: testID(802), Layer: cellar, Color: "#0000ff", Width: 2, Points: []int{4, 4}}, w.gm)

	w.apply(&RoomSetLocked{Locked: true}, w.gm)

	return w
}
