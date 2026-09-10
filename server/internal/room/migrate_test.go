package room

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// EVERY PAST SCHEMA HAS A GOLDEN SNAPSHOT IN testdata/snapshots, AND EVERY ONE
// OF THEM HAS TO DECODE TO TODAY'S SHAPE. That is the test that turns "bump the
// number" into "bump the number and write the step": a Schema of 3 with no
// schema-2.json beside it, or one with no migration to read it, fails here
// rather than throwing every room's table away on the deploy.
func TestEveryPastSchemaMigratesToTheCurrentOne(t *testing.T) {
	for n := 1; n <= Schema; n++ {
		name := filepath.Join("testdata", "snapshots", "schema-"+itoa(n)+".json")
		t.Run(name, func(t *testing.T) {
			blob, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("every schema up to %d needs a golden snapshot: %v", Schema, err)
			}

			s, err := Unmarshal(blob)
			if err != nil {
				t.Fatalf("a schema %d snapshot no longer decodes: %v", n, err)
			}
			if s.Schema != Schema {
				t.Errorf("schema = %d after migrating, want %d", s.Schema, Schema)
			}

			// And what came back is a room, with the pawns where they were:
			// migrating to an empty table would pass the checks above and be
			// exactly the loss a migration exists to prevent.
			if len(s.Pawns) == 0 || len(s.Table.Layers) == 0 {
				t.Errorf("the migrated room has %d pawns on %d layers; the fixture has both", len(s.Pawns), len(s.Table.Layers))
			}

			// Marshalling the result writes today's schema, so a second read is
			// the ordinary path with no migration in it.
			again, err := Marshal(s)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if _, err := Unmarshal(again); err != nil {
				t.Errorf("the migrated snapshot does not round-trip: %v", err)
			}
		})
	}
}

// Schema 1 measured an object in cells and schema 2 measures it in map pixels;
// the step multiplies by the room's cell size, so a two-by-four wagon on a
// fifty-pixel grid comes back a hundred by two hundred rather than two by four
// -- which would have been an invisible wagon.
func TestSchemaOneFootprintsBecomePixels(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "snapshots", "schema-1.json"))
	if err != nil {
		t.Fatal(err)
	}

	s, err := Unmarshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range s.Pawns {
		switch p.Name {
		case "Wagon":
			if p.Width != 100 || p.Height != 200 {
				t.Errorf("the wagon is %dx%d pixels, want 100x200", p.Width, p.Height)
			}
			if p.Rotation != 0 {
				t.Errorf("rotation = %d, want 0", p.Rotation)
			}
		case "Goblin":
			if p.Width != 0 || p.Height != 0 || p.Size != SizeSmall {
				t.Errorf("the goblin is %dx%d %q; a creature keeps its size and has no pixel size", p.Width, p.Height, p.Size)
			}
		}
	}

	// The two schema-1 fields that Normalize repairs came through it as well.
	if s.Table.PawnLabels != LabelsDefault || s.Table.Grid.Lines != GridLinesSolid {
		t.Errorf("labels = %q, lines = %q; the repairs in Normalize did not run", s.Table.PawnLabels, s.Table.Grid.Lines)
	}
}

// Schema 2 had no stroke kinds because a stroke could only be one thing. An
// empty kind is not a member of the set, so a room restored without the step
// would come back holding drawing that fails Valid() and that nothing knows how
// to draw -- and it would do it silently, which is what the step prevents.
func TestSchemaTwoStrokesBecomeFreehand(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "snapshots", "schema-2.json"))
	if err != nil {
		t.Fatal(err)
	}

	s, err := Unmarshal(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Strokes) == 0 {
		t.Fatal("the schema-2 fixture has no strokes in it, so this proves nothing")
	}

	for _, st := range s.Strokes {
		if st.Kind != StrokeFree {
			t.Errorf("stroke %s came back %q, want %q", st.ID, st.Kind, StrokeFree)
		}

		// The rest of the stroke came through untouched: a step that repaired
		// the kind and dropped the line would pass the check above.
		if len(st.Points) == 0 || st.Color == "" || st.Width == 0 {
			t.Errorf("stroke %s lost its points, colour or width: %+v", st.ID, st)
		}
	}

	// AND DONE IS LEFT ALONE. A free stroke is finished when its author lifts
	// the pen, and the fixture holds one of each -- so a step that set Done
	// along with the kind would have quietly closed a line somebody was still
	// drawing when the server went down.
	var open int
	for _, st := range s.Strokes {
		if !st.Done {
			open++
		}
	}
	if open != 1 {
		t.Errorf("%d strokes came back unfinished, want 1", open)
	}
}

// A snapshot from a schema this build has never seen is a rollback, and the
// answer is ErrSchema -- which the hub treats as "start fresh and keep the
// bytes" rather than as something to guess at.
func TestASnapshotFromTheFutureIsRefusedNotGuessedAt(t *testing.T) {
	blob := []byte(`{"schema":` + itoa(Schema+1) + `,"seq":7,"room":{"id":"","name":"later","locked":false}}`)

	_, err := Unmarshal(blob)
	if !errors.Is(err, ErrSchema) {
		t.Fatalf("error = %v, want ErrSchema", err)
	}
}

// The golden snapshot for the CURRENT schema is regenerated on -update, from
// the same world the other fixtures come from, so that bumping Schema is the
// moment the previous shape gets pinned.
func TestTheCurrentSchemaHasAGoldenSnapshot(t *testing.T) {
	name := filepath.Join("testdata", "snapshots", "schema-"+itoa(Schema)+".json")

	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", X: 64, Y: 64, Visible: true})
	w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", X: 200, Y: 200, Width: 128, Height: 256, Visible: true})

	// THE FIXTURE CARRIES DRAWING, and it does because schema 2's did not: the
	// step that gave a stroke a kind had nothing to run against until strokes
	// were added to that fixture by hand. A golden snapshot is only worth what
	// the next migration can be tested on, so every collection that a step
	// might one day have to touch has something in it here.
	w.apply(&StrokeBegin{ID: testID(900), Layer: w.layer, Kind: StrokeFree, Color: "#FF0000FF", Width: 4, Points: []int{0, 0, 8, 8}}, w.gm)
	w.apply(&StrokeEnd{ID: testID(900)}, w.gm)
	w.apply(&StrokeBegin{ID: testID(901), Layer: w.layer, Kind: StrokeCircle, Color: "#00FF00FF", Width: 2, Points: []int{128, 128, 256, 128}}, w.gm)
	w.apply(&StrokeBegin{ID: testID(902), Layer: w.layer, Kind: StrokeFree, Color: "#0000FFFF", Width: 2, Points: []int{4, 4}}, w.pc)

	blob, err := json.MarshalIndent(w.s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	blob = append(blob, '\n')

	if *update {
		if err := os.WriteFile(name, blob, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := os.Stat(name); err != nil {
		t.Fatalf("schema %d has no golden snapshot; run the tests with -update to write it", Schema)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
