package room

import (
	"reflect"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestTheGMsChangesShareNoMemoryWithTheRoom(t *testing.T) {
	for name, act := range map[string]func(w *world) change{
		"a spawned pawn": func(w *world) change {
			return w.change(&PawnSpawn{
				Kind: PawnMonster, Layer: w.layer, X: 64, Y: 64, Visible: true,
				Pawn: &Pawn{Name: "Goblin", Size: SizeSmall, HP: intp(5), MaxHP: intp(7), AC: intp(15)},
			}, w.gm)
		},
		"a pawn's conditions": func(w *world) change {
			goblin := w.spawn(Pawn{Name: "Goblin", Visible: true, HP: intp(5), MaxHP: intp(7)})
			return w.change(&PawnSetConditions{ID: goblin, Conditions: []Condition{
				{Name: "Poisoned", Color: ColorGreen, Duration: -1, Clear: ClearEnd},
			}}, w.gm)
		},
		"a moved pawn": func(w *world) change {
			goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
			return w.change(&PawnMove{Anchor: goblin, X: 192, Y: 192}, w.gm)
		},
		"a growing stroke": func(w *world) change {
			w.apply(&StrokeBegin{ID: testID(700), Layer: w.layer, Kind: StrokeFree, Color: "#ff0000ff", Width: 4, Points: []int{0, 0, 10, 10}}, w.gm)
			return w.change(&StrokeExtend{ID: testID(700), Points: []int{20, 20, 30, 30}}, w.gm)
		},
		"a fog shape": func(w *world) change {
			return w.change(&FogAdd{Layer: w.layer, Kind: ShapeRect, Mode: FogHide, Points: []int{0, 0, 64, 64}}, w.gm)
		},
		"a party start": func(w *world) change {
			return w.change(&TableSetPartyStart{Layer: w.layer, X: intp(64), Y: intp(64)}, w.gm)
		},
		"the bag": func(w *world) change {
			lib := newLibrary()
			lib.pictures[pictureKey{id: testTerrainID, kind: PictureTerrain}] = pines()
			cmd := &PaletteAdd{Asset: testTerrainID}
			w.resolve(cmd, lib)
			return w.change(cmd, w.gm)
		},
		"a stamp": func(w *world) change {
			art := w.addArt(testTerrainID, pines())
			return w.change(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}, {Q: 1, R: 0}}}, w.gm)
		},
		"the initiative": func(w *world) change {
			goblin := w.spawn(Pawn{Name: "Goblin", Visible: true})
			return w.change(&InitiativeSet{Entries: []InitiativeEntry{{Name: "Goblin", PawnIDs: []ulid.ULID{goblin}}}}, w.gm)
		},
		"a roll": func(w *world) change {
			return w.change(&DiceRoll{Expr: "2d6"}, w.pc)
		},
		"a seat": func(w *world) change {
			return w.change(&PlayerJoin{Player: Player{ID: testID(900), Name: "Sam", Role: RolePlayer, CharacterID: idp(testID(901)), CharacterName: "Tam"}}, w.gm)
		},
		"the music": func(w *world) change {
			return w.change(&MusicLoad{AssetID: testTrackID, Track: &TrackInfo{Name: "Tavern Brawl"}}, w.gm)
		},
		"the room": func(w *world) change {
			return w.change(&RoomSetName{Name: "The Sunless Citadel, again"}, w.gm)
		},
		"the table": func(w *world) change {
			return w.change(&TableSetActiveLayer{Layer: w.addLayer("Cellar")}, w.gm)
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			changes := act(w).changes(RoleGM)
			if len(changes) == 0 {
				t.Fatal("the GM was told nothing")
			}
			was := encodeChanges(t, changes)
			scribble(reflect.ValueOf(w.s).Elem())
			if now := encodeChanges(t, changes); now != was {
				t.Errorf("writing over the room rewrote what the GM was sent:\nwas %s\nnow %s", was, now)
			}
		})
	}
}
func encodeChanges(t *testing.T, changes []Change) string {
	t.Helper()
	raw, err := EncodeEvent(NewChanges(changes), 1, nil)
	if err != nil {
		t.Fatalf("encoding the changes: %v", err)
	}
	return string(raw)
}
func scribble(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			scribble(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				scribble(v.Field(i))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			scribble(v.Index(i))
		}
	case reflect.String:
		v.SetString("scribbled")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(-7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(-7)
	}
}
func BenchmarkDeriveStrokeExtend(b *testing.B) {
	before := NewState(testRoomID, "The Sunless Citadel", newEnv())
	for n := range 20 {
		points := make([]int, 300)
		for i := range points {
			points[i] = n + i
		}
		before.Strokes = append(before.Strokes, Stroke{
			ID: testID(800 + n), By: testGMID, LayerID: before.Table.ActiveLayer,
			Kind: StrokeFree, Color: "#ff0000ff", Width: 4, Points: points,
		})
	}
	before.Normalize()
	after := before.Clone()
	growing := after.Stroke(testID(800))
	growing.Points = append(growing.Points, 1, 2, 3, 4)
	for _, role := range []Role{RoleGM, RolePlayer} {
		b.Run(string(role), func(b *testing.B) {
			for b.Loop() {
				Derive(before, &after, role)
			}
		})
	}
}
