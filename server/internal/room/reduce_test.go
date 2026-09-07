package room

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

// EVERY EVENT IN THE REGISTRY HAS A REDUCTION. This walks the catalog rather
// than listing the cases, so an event added without deciding what it does to
// the state fails here instead of being silently ignored by every client.
func TestEveryEventReduces(t *testing.T) {
	for wire, ev := range EventPrototypes() {
		s := NewState(testRoomID, "Room", newEnv())

		if err := Reduce(s, ev); err != nil {
			t.Errorf("%s: %v", wire, err)
		}
	}
}

// The default branch is not decoration. It is what turns "somebody added an
// event and forgot the reducer" into a failure with the type name in it.
func TestReduceNamesAnEventItDoesNotKnow(t *testing.T) {
	s := NewState(testRoomID, "Room", newEnv())

	err := Reduce(s, &unreducedEvent{})
	if err == nil {
		t.Fatal("an event with no reduction was accepted")
	}
	if !strings.Contains(err.Error(), "an.event.nobody.wrote") {
		t.Fatalf("the error does not name the event: %v", err)
	}
}

type unreducedEvent struct{ Header }

func (*unreducedEvent) eventType() string { return "an.event.nobody.wrote" }

// A transient event never touches the state. If one ever did, it would be a
// change that the snapshot does not carry, and the room would come back from a
// restart missing it.
func TestTransientEventsChangeNothing(t *testing.T) {
	w := busyWorld(t)
	before := mustJSON(t, w.s)

	transient := []Event{
		&RoomClosed{},
		&PlayerKicked{Reason: "gone"},
		&PawnDragging{Pawns: []PawnPosition{{ID: w.s.Pawns[0].ID, X: 9999, Y: 9999}}},
		&Pinged{Layer: w.layer, X: 5, Y: 5},
		&ErrorEvent{Code: CodeInvalid},
	}

	for _, ev := range transient {
		if err := Reduce(w.s, ev); err != nil {
			t.Fatalf("%s: %v", ev.eventType(), err)
		}
	}

	if got := mustJSON(t, w.s); got != before {
		t.Fatal("a transient event changed the state")
	}
}

// The three sizing rules, each checked as the shape it is. A singleton
// replaces, a collection item upserts by id, and a hot path mutates a named
// field of something already there.
func TestTheReducerAppliesTheThreeSizingRules(t *testing.T) {
	s := NewState(testRoomID, "Room", newEnv())
	layer := s.Table.ActiveLayer

	t.Run("a singleton replaces", func(t *testing.T) {
		if err := Reduce(s, &RoomUpdated{Room: RoomInfo{ID: testRoomID, Name: "Renamed", Locked: true}}); err != nil {
			t.Fatal(err)
		}
		if s.Room.Name != "Renamed" || !s.Room.Locked {
			t.Fatalf("the room is %+v", s.Room)
		}
	})

	t.Run("a collection item upserts by id", func(t *testing.T) {
		pawn := Pawn{ID: testID(1), Kind: PawnMonster, LayerID: layer, Name: "Goblin", Size: SizeSmall, Visible: true}

		if err := Reduce(s, &PawnSpawned{Pawn: pawn}); err != nil {
			t.Fatal(err)
		}
		pawn.Name = "Goblin boss"
		if err := Reduce(s, &PawnUpdated{Pawn: pawn}); err != nil {
			t.Fatal(err)
		}

		if len(s.Pawns) != 1 {
			t.Fatalf("the update added a second pawn: %d", len(s.Pawns))
		}
		if s.Pawns[0].Name != "Goblin boss" {
			t.Fatalf("the pawn is called %q", s.Pawns[0].Name)
		}

		if err := Reduce(s, &PawnRemoved{ID: testID(1)}); err != nil {
			t.Fatal(err)
		}
		if len(s.Pawns) != 0 {
			t.Fatal("the pawn was not removed")
		}
	})

	t.Run("a hot path mutates what is already there", func(t *testing.T) {
		if err := Reduce(s, &PawnSpawned{Pawn: Pawn{ID: testID(2), Kind: PawnMonster, LayerID: layer, Size: SizeMedium}}); err != nil {
			t.Fatal(err)
		}
		if err := Reduce(s, &PawnMoved{Pawns: []PawnPosition{{ID: testID(2), X: 128, Y: 64}}}); err != nil {
			t.Fatal(err)
		}
		if p := s.Pawn(testID(2)); p.X != 128 || p.Y != 64 {
			t.Fatalf("the pawn is at (%d, %d)", p.X, p.Y)
		}

		// A move naming a pawn this client does not hold is ignored rather
		// than an error: the player copy of a move is filtered per audience,
		// and a removal can race an event either way round.
		if err := Reduce(s, &PawnMoved{Pawns: []PawnPosition{{ID: testID(999), X: 1, Y: 1}}}); err != nil {
			t.Fatalf("a move naming an absent pawn was an error: %v", err)
		}

		if err := Reduce(s, &StrokeBegan{Stroke: Stroke{ID: testID(3), LayerID: layer, Points: []int{0, 0}}}); err != nil {
			t.Fatal(err)
		}
		if err := Reduce(s, &StrokeExtended{ID: testID(3), Points: []int{8, 8}}); err != nil {
			t.Fatal(err)
		}
		if got := s.Stroke(testID(3)).Points; len(got) != 4 {
			t.Fatalf("the stroke holds %v after one extension", got)
		}
	})

	t.Run("fog keeps the order it arrived in", func(t *testing.T) {
		s.Fog = nil
		for _, n := range []int{9, 4, 7} {
			if err := Reduce(s, &FogAdded{Shape: FogShape{ID: testID(n), LayerID: layer, Kind: ShapeRect, Points: []int{0, 0, 1, 1}}}); err != nil {
				t.Fatal(err)
			}
		}

		want := []ulid.ULID{testID(9), testID(4), testID(7)}
		for i, id := range want {
			if s.Fog[i].ID != id {
				t.Fatal("the reducer sorted the fog; a hide drawn over a reveal would now be under it")
			}
		}
	})
}

// A snapshot replaces everything, which is what makes reconnecting simple
// enough that there is no replay log behind it.
func TestASnapshotEventReplacesTheWholeState(t *testing.T) {
	w := busyWorld(t)

	client := NewState(testRoomID, "Something else", newEnv())
	client.Pawns = append(client.Pawns, Pawn{ID: testID(77), Kind: PawnMonster, Size: SizeMedium})
	client.Normalize()

	if err := Reduce(client, &Snapshot{State: w.s.Project(RolePlayer)}); err != nil {
		t.Fatal(err)
	}

	if mustJSON(t, client) != mustJSON(t, w.s.Project(RolePlayer)) {
		t.Fatal("the client did not end up holding the snapshot it was sent")
	}
}
