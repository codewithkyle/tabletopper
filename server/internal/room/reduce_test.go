package room

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestEveryChangeReduces(t *testing.T) {
	for wire, ch := range ChangePrototypes() {
		s := NewState(testRoomID, "Room", newEnv())
		if err := Reduce(s, ch); err != nil {
			t.Errorf("%s: %v", wire, err)
		}
	}
}
func TestReduceNamesAChangeItDoesNotKnow(t *testing.T) {
	s := NewState(testRoomID, "Room", newEnv())
	err := Reduce(s, &unreducedChange{})
	if err == nil {
		t.Fatal("a change with no reduction was accepted")
	}
	if !strings.Contains(err.Error(), "a.change.nobody.wrote") {
		t.Fatalf("the error does not name the change: %v", err)
	}
}

type unreducedChange struct{ Kind }

func (*unreducedChange) changeType() string { return "a.change.nobody.wrote" }
func TestTheGenericReducerUpsertsAndRemovesEveryCollection(t *testing.T) {
	layer := testID(400)
	cases := []struct {
		name     string
		upsert   func(n int) Change
		again    func(n int) Change
		remove   func(ids ...ulid.ULID) Change
		count    func(s *State) int
		firstID  func(s *State) ulid.ULID
		renamed  func(s *State) string
		wantName string
	}{
		{
			name: "players",
			upsert: func(n int) Change {
				return &PlayersUpserted{Players: []Player{{ID: testID(n), Name: "Ari"}, {ID: testID(n + 1), Name: "Rin"}}}
			},
			again:  func(n int) Change { return &PlayersUpserted{Players: []Player{{ID: testID(n), Name: "Ari the Bold"}}} },
			remove: func(ids ...ulid.ULID) Change { return &PlayersRemoved{IDs: ids} },
			count:  func(s *State) int { return len(s.Players) },
			firstID: func(s *State) ulid.ULID {
				return s.Players[0].ID
			},
			renamed:  func(s *State) string { return s.Players[0].Name },
			wantName: "Ari the Bold",
		},
		{
			name: "pawns",
			upsert: func(n int) Change {
				return &PawnsUpserted{Pawns: []Pawn{
					{ID: testID(n), Kind: PawnMonster, LayerID: layer, Name: "Goblin", Size: SizeSmall},
					{ID: testID(n + 1), Kind: PawnMonster, LayerID: layer, Name: "Ogre", Size: SizeLarge},
				}}
			},
			again: func(n int) Change {
				return &PawnsUpserted{Pawns: []Pawn{{ID: testID(n), Kind: PawnMonster, LayerID: layer, Name: "Goblin boss", Size: SizeSmall}}}
			},
			remove:   func(ids ...ulid.ULID) Change { return &PawnsRemoved{IDs: ids} },
			count:    func(s *State) int { return len(s.Pawns) },
			firstID:  func(s *State) ulid.ULID { return s.Pawns[0].ID },
			renamed:  func(s *State) string { return s.Pawns[0].Name },
			wantName: "Goblin boss",
		},
		{
			name: "fog",
			upsert: func(n int) Change {
				return &FogUpserted{Shapes: []FogShape{
					{ID: testID(n), LayerID: layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 1, 1}},
					{ID: testID(n + 1), LayerID: layer, Kind: ShapeRect, Mode: FogHide, Points: []int{2, 2, 3, 3}},
				}}
			},
			again: func(n int) Change {
				return &FogUpserted{Shapes: []FogShape{{ID: testID(n), LayerID: layer, Kind: ShapeRect, Mode: FogReveal, Points: []int{0, 0, 1, 1}}}}
			},
			remove:   func(ids ...ulid.ULID) Change { return &FogRemoved{IDs: ids} },
			count:    func(s *State) int { return len(s.Fog) },
			firstID:  func(s *State) ulid.ULID { return s.Fog[0].ID },
			renamed:  func(s *State) string { return string(s.Fog[0].Mode) },
			wantName: string(FogReveal),
		},
		{
			name: "strokes",
			upsert: func(n int) Change {
				return &StrokesUpserted{Strokes: []Stroke{
					{ID: testID(n), LayerID: layer, Kind: StrokeFree, Points: []int{0, 0}},
					{ID: testID(n + 1), LayerID: layer, Kind: StrokeFree, Points: []int{1, 1}},
				}}
			},
			again: func(n int) Change {
				return &StrokesUpserted{Strokes: []Stroke{{ID: testID(n), LayerID: layer, Kind: StrokeFree, Color: "#ff0000ff", Points: []int{0, 0}}}}
			},
			remove:   func(ids ...ulid.ULID) Change { return &StrokesRemoved{IDs: ids} },
			count:    func(s *State) int { return len(s.Strokes) },
			firstID:  func(s *State) ulid.ULID { return s.Strokes[0].ID },
			renamed:  func(s *State) string { return s.Strokes[0].Color },
			wantName: "#ff0000ff",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState(testRoomID, "Room", newEnv())
			s.Fog = nil
			if err := Reduce(s, tc.upsert(1)); err != nil {
				t.Fatal(err)
			}
			if got := tc.count(s); got != 2 {
				t.Fatalf("an upsert of two left %d behind", got)
			}
			if err := Reduce(s, tc.again(1)); err != nil {
				t.Fatal(err)
			}
			if got := tc.count(s); got != 2 {
				t.Fatalf("an upsert of one that was already there left %d behind", got)
			}
			if got := tc.renamed(s); got != tc.wantName {
				t.Fatalf("the upserted copy reads %q, want %q", got, tc.wantName)
			}
			if err := Reduce(s, tc.remove(tc.firstID(s))); err != nil {
				t.Fatal(err)
			}
			if got := tc.count(s); got != 1 {
				t.Fatalf("a removal of one left %d behind", got)
			}
			if err := Reduce(s, tc.remove(testID(9999))); err != nil {
				t.Fatalf("a removal naming nothing that is there was an error: %v", err)
			}
			if got := tc.count(s); got != 1 {
				t.Fatalf("a removal naming nothing that is there left %d behind", got)
			}
		})
	}
}
func TestTransientEventsAreNotChanges(t *testing.T) {
	for wire, ev := range EventPrototypes() {
		if _, isChange := ev.(Change); isChange {
			t.Errorf("the frame %s is also a change; the two vocabularies must not overlap", wire)
		}
	}
	for wire, ch := range ChangePrototypes() {
		if _, isEvent := ch.(Event); isEvent {
			t.Errorf("the change %s is also a frame", wire)
		}
	}
}
func TestTheAggregatesReplaceAndTheDeltasMutate(t *testing.T) {
	s := NewState(testRoomID, "Room", newEnv())
	layer := s.Table.ActiveLayer
	t.Run("an aggregate replaces", func(t *testing.T) {
		if err := Reduce(s, &RoomUpdated{Room: RoomInfo{ID: testRoomID, Name: "Renamed", Locked: true}}); err != nil {
			t.Fatal(err)
		}
		if s.Room.Name != "Renamed" || !s.Room.Locked {
			t.Fatalf("the room is %+v", s.Room)
		}
		settings := s.Table.TableSettings
		settings.PawnLabels = LabelsFull
		if err := Reduce(s, &TableUpdated{Table: settings}); err != nil {
			t.Fatal(err)
		}
		if s.Table.PawnLabels != LabelsFull {
			t.Fatalf("the label setting is %q", s.Table.PawnLabels)
		}
		if len(s.Table.Layers) != 1 {
			t.Fatal("a table change dropped the layers, which travel on their own")
		}
	})
	t.Run("the layer list replaces", func(t *testing.T) {
		second := testID(401)
		if err := Reduce(s, &LayersUpdated{Layers: []Layer{{ID: second, Name: "Cellar"}, {ID: layer, Name: DefaultLayerName}}}); err != nil {
			t.Fatal(err)
		}
		if len(s.Table.Layers) != 2 || s.Table.Layers[0].ID != second {
			t.Fatalf("the layer list is %+v; the order it arrived in is the order it keeps", s.Table.Layers)
		}
	})
	t.Run("a delta mutates what is already there", func(t *testing.T) {
		if err := Reduce(s, &PawnsUpserted{Pawns: []Pawn{{ID: testID(2), Kind: PawnMonster, LayerID: layer, Size: SizeMedium}}}); err != nil {
			t.Fatal(err)
		}
		if err := Reduce(s, &PawnsMoved{Pawns: []PawnPosition{{ID: testID(2), X: 128, Y: 64}}}); err != nil {
			t.Fatal(err)
		}
		if p := s.Pawn(testID(2)); p.X != 128 || p.Y != 64 {
			t.Fatalf("the pawn is at (%d, %d)", p.X, p.Y)
		}
		if err := Reduce(s, &PawnsMoved{Pawns: []PawnPosition{{ID: testID(999), X: 1, Y: 1}}}); err != nil {
			t.Fatalf("a move naming an absent pawn was an error: %v", err)
		}
		if err := Reduce(s, &StrokesUpserted{Strokes: []Stroke{{ID: testID(3), LayerID: layer, Points: []int{0, 0}}}}); err != nil {
			t.Fatal(err)
		}
		if err := Reduce(s, &StrokeExtended{ID: testID(3), Points: []int{8, 8}}); err != nil {
			t.Fatal(err)
		}
		if got := s.Stroke(testID(3)).Points; len(got) != 4 {
			t.Fatalf("the stroke holds %v after one extension", got)
		}
		if err := Reduce(s, &StrokeEnded{ID: testID(3)}); err != nil {
			t.Fatal(err)
		}
		if !s.Stroke(testID(3)).Done {
			t.Fatal("the stroke was not finished")
		}
	})
	t.Run("fog keeps the order it arrived in", func(t *testing.T) {
		s.Fog = nil
		for _, n := range []int{9, 4, 7} {
			if err := Reduce(s, &FogUpserted{Shapes: []FogShape{{ID: testID(n), LayerID: layer, Kind: ShapeRect, Points: []int{0, 0, 1, 1}}}}); err != nil {
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
func TestASnapshotReplacesTheWholeState(t *testing.T) {
	w := busyWorld(t)
	client := NewState(testRoomID, "Something else", newEnv())
	client.Pawns = append(client.Pawns, Pawn{ID: testID(77), Kind: PawnMonster, Size: SizeMedium})
	client.Normalize()
	if err := replay(client, &Snapshot{State: w.s.Project(RolePlayer)}); err != nil {
		t.Fatal(err)
	}
	if mustJSON(t, client) != mustJSON(t, w.s.Project(RolePlayer)) {
		t.Fatal("the client did not end up holding the snapshot it was sent")
	}
}
