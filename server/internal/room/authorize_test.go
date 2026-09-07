package room

import (
	"slices"
	"testing"

	"github.com/oklog/ulid/v2"
)

// THE AUTHORIZATION TABLE. One row per wire command, one column per kind of
// actor, one assertion per cell.
//
// IT IS A TABLE ON PURPOSE. Authority is the part of a protocol where a rule
// written twice eventually gets changed once, and reading thirty Authorize
// methods to answer "what can a player do" is how a permission gets granted by
// accident. Here the answer is a column.
//
// THE THREE ACTORS ARE THE THREE THAT EXIST: the GM, the player who owns the
// thing being named, and another player who does not. Everything in this
// package's authority rules is one of those three.
func TestAuthorizeCoversEveryWireCommand(t *testing.T) {
	w, fx := authorizeWorld(t)

	const ok = ""

	tests := []struct {
		wire  string
		cmd   Command
		gm    string
		owner string
		other string
	}{
		{"table.addLayer", &TableAddLayer{Name: "Cellar"}, ok, CodeForbidden, CodeForbidden},
		{"table.removeLayer", &TableRemoveLayer{Layer: fx.spare}, ok, CodeForbidden, CodeForbidden},
		{"table.renameLayer", &TableRenameLayer{Layer: fx.spare, Name: "Attic"}, ok, CodeForbidden, CodeForbidden},
		{"table.moveLayer", &TableMoveLayer{Layer: fx.spare, Index: 0}, ok, CodeForbidden, CodeForbidden},
		{"table.setLayerMap", &TableSetLayerMap{Layer: fx.spare, AssetID: testAssetID}, ok, CodeForbidden, CodeForbidden},
		{"table.clearLayerMap", &TableClearLayerMap{Layer: fx.spare}, ok, CodeForbidden, CodeForbidden},
		{"table.setActiveLayer", &TableSetActiveLayer{Layer: fx.spare}, ok, CodeForbidden, CodeForbidden},
		{"table.setGrid", &TableSetGrid{Grid: w.s.Table.Grid}, ok, CodeForbidden, CodeForbidden},
		{"table.setOptions", &TableSetOptions{MonsterHP: HPExact}, ok, CodeForbidden, CodeForbidden},

		// A player may place exactly one thing: the character they joined
		// with, on the layer everybody is looking at. The other player is
		// refused because it is not their character, not because of their role.
		{"pawn.spawn", &PawnSpawn{Kind: PawnPlayer, Layer: w.layer, CharacterID: &testCharID}, ok, ok, CodeForbidden},
		{"pawn.spawnCharacters", &PawnSpawnCharacters{}, ok, CodeForbidden, CodeForbidden},
		{"pawn.move", &PawnMove{Anchor: fx.owned}, ok, ok, CodeForbidden},
		{"pawn.drag", &PawnDrag{Anchor: fx.owned}, ok, ok, CodeForbidden},
		{"pawn.update", &PawnUpdate{ID: fx.owned}, ok, ok, CodeForbidden},
		{"pawn.setConditions", &PawnSetConditions{ID: fx.owned}, ok, ok, CodeForbidden},
		{"pawn.setVisible", &PawnSetVisible{ID: fx.owned}, ok, CodeForbidden, CodeForbidden},
		{"pawn.setLayer", &PawnSetLayer{IDs: []ulid.ULID{fx.owned}, Layer: fx.spare}, ok, CodeForbidden, CodeForbidden},
		{"pawn.remove", &PawnRemove{IDs: []ulid.ULID{fx.owned}}, ok, CodeForbidden, CodeForbidden},

		{"initiative.set", &InitiativeSet{}, ok, CodeForbidden, CodeForbidden},

		// The one command whose player column depends on the state rather than
		// on ownership of a thing named in it: whoever's turn it is may end it.
		{"initiative.next", &InitiativeNext{}, ok, ok, CodeForbidden},
		{"initiative.clear", &InitiativeClear{}, ok, CodeForbidden, CodeForbidden},

		{"fog.setEnabled", &FogSetEnabled{Layer: w.layer}, ok, CodeForbidden, CodeForbidden},
		{"fog.setPrefill", &FogSetPrefill{Layer: w.layer}, ok, CodeForbidden, CodeForbidden},
		{"fog.add", &FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 64, 64}}, ok, CodeForbidden, CodeForbidden},
		{"fog.remove", &FogRemove{ID: fx.shape}, ok, CodeForbidden, CodeForbidden},
		{"fog.clear", &FogClear{Layer: w.layer}, ok, CodeForbidden, CodeForbidden},

		// Drawing is a room setting rather than a role, so all three may begin
		// a stroke while the setting is on.
		{"stroke.begin", &StrokeBegin{ID: testID(500), Layer: w.layer, Color: "#ffffff", Width: 2, Points: []int{0, 0}}, ok, ok, ok},

		// Extending and ending are own-stroke, and that includes the GM: the
		// person drawing the line is still drawing it, and taking it away is
		// what erase is for.
		{"stroke.extend", &StrokeExtend{ID: fx.stroke}, CodeForbidden, ok, CodeForbidden},
		{"stroke.end", &StrokeEnd{ID: fx.stroke}, CodeForbidden, ok, CodeForbidden},
		{"stroke.erase", &StrokeErase{IDs: []ulid.ULID{fx.stroke}}, ok, ok, CodeForbidden},
		{"stroke.clear", &StrokeClear{Layer: w.layer}, ok, CodeForbidden, CodeForbidden},

		{"ping", &Ping{Layer: w.layer}, ok, ok, ok},
		{"player.kick", &PlayerKick{ID: testOtherID}, ok, CodeForbidden, CodeForbidden},
		{"sync.request", &SyncRequest{}, ok, ok, ok},
	}

	// Nothing may be left out. A command added to the registry without a row
	// here is a command whose authority nobody wrote down.
	covered := map[string]bool{}
	for _, tc := range tests {
		covered[tc.wire] = true
	}
	for wire := range WireCommandPrototypes() {
		if !covered[wire] {
			t.Errorf("%s has no row in the authorization table", wire)
		}
	}

	for _, tc := range tests {
		for _, cell := range []struct {
			who   string
			actor Actor
			want  string
		}{
			{"the GM", w.gm, tc.gm},
			{"the owner", w.pc, tc.owner},
			{"another player", w.other, tc.other},
		} {
			err := tc.cmd.Authorize(w.s, cell.actor)

			if cell.want == ok {
				if err != nil {
					t.Errorf("%s: %s was refused: %v", tc.wire, cell.who, err)
				}

				continue
			}

			if err == nil {
				t.Errorf("%s: %s was allowed, want %s", tc.wire, cell.who, cell.want)

				continue
			}
			if e, isRoomError := err.(*Error); !isRoomError || e.Code != cell.want {
				t.Errorf("%s: %s was refused with %v, want %s", tc.wire, cell.who, err, cell.want)
			}
		}
	}
}

// authorizeFixture is the handful of ids the table above names.
type authorizeFixture struct {
	spare  ulid.ULID
	owned  ulid.ULID
	stroke ulid.ULID
	shape  ulid.ULID
}

func authorizeWorld(t *testing.T) (*world, authorizeFixture) {
	t.Helper()

	w := newWorld(t)
	fx := authorizeFixture{spare: w.addLayer("Cellar")}

	fx.owned = w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: &testPlayerID, CharacterID: &testCharID})

	fx.stroke = testID(400)
	w.apply(&StrokeBegin{ID: fx.stroke, Layer: w.layer, Color: "#ff0000", Width: 3, Points: []int{0, 0, 10, 10}}, w.pc)

	w.apply(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 100, 100}}, w.gm)
	fx.shape = w.s.Fog[0].ID

	// The owner's pawn is up in the tracker, which is what makes the
	// initiative.next row's middle column mean anything.
	w.apply(&InitiativeSet{
		Entries: []InitiativeEntry{{Name: "Ari", PawnID: &fx.owned, Initiative: 18}},
	}, w.gm)
	w.apply(&InitiativeNext{}, w.gm)

	return w, fx
}

// Authorize is a question, not a change. Every Apply in this package assumes it
// ran first and did nothing, and a rule that peeked by mutating would make the
// refusal path leave the room in a state nobody asked for.
func TestAuthorizeNeverMutates(t *testing.T) {
	w, fx := authorizeWorld(t)

	before := mustJSON(t, w.s)

	for wire, cmd := range WireCommandPrototypes() {
		for _, a := range []Actor{w.gm, w.pc, w.other} {
			_ = cmd.Authorize(w.s, a)
		}
		if got := mustJSON(t, w.s); got != before {
			t.Fatalf("%s changed the state from its Authorize", wire)
		}
	}

	// And the same for the commands only the hub builds, since the hub calls
	// Authorize on those too rather than special-casing them.
	for wire, cmd := range HubCommandPrototypes() {
		_ = cmd.Authorize(w.s, w.gm)
		if got := mustJSON(t, w.s); got != before {
			t.Fatalf("%s changed the state from its Authorize", wire)
		}
	}

	_ = fx
}

// The GM cannot be kicked and cannot kick themselves. A room whose owner has
// left it is a room nobody can unlock, close or reopen.
func TestTheGMCannotBeRemoved(t *testing.T) {
	w := newWorld(t)

	w.refuse(&PlayerKick{ID: testGMID}, w.gm, CodeForbidden)

	// The refusal for a GM naming themselves comes from Authorize, and the one
	// for a second GM row would come from Apply; both are forbidden and both
	// are checked, because only the second survives if the first is ever
	// relaxed.
	if err := (&PlayerKick{ID: testGMID}).Authorize(w.s, Actor{ID: testPlayerID, Role: RoleGM}); err != nil {
		t.Fatalf("a GM naming somebody else was refused by Authorize: %v", err)
	}
	if _, err := (&PlayerKick{ID: testGMID}).Apply(w.s, Actor{ID: testPlayerID, Role: RoleGM}, w.env); err == nil {
		t.Fatal("Apply removed the GM's own row")
	}

	if !slices.ContainsFunc(w.s.Players, func(p Player) bool { return p.ID == testGMID }) {
		t.Fatal("the GM is no longer in the room")
	}
}
