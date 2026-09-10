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
