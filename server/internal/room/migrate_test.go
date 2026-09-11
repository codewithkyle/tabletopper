package room

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)






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

			
			
			
			if len(s.Pawns) == 0 || len(s.Table.Layers) == 0 {
				t.Errorf("the migrated room has %d pawns on %d layers; the fixture has both", len(s.Pawns), len(s.Table.Layers))
			}

			
			
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

	
	if s.Table.PawnLabels != LabelsDefault || s.Table.Grid.Lines != GridLinesSolid {
		t.Errorf("labels = %q, lines = %q; the repairs in Normalize did not run", s.Table.PawnLabels, s.Table.Grid.Lines)
	}
}





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

		
		
		if len(st.Points) == 0 || st.Color == "" || st.Width == 0 {
			t.Errorf("stroke %s lost its points, colour or width: %+v", st.ID, st)
		}
	}

	
	
	
	
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




func TestASnapshotFromTheFutureIsRefusedNotGuessedAt(t *testing.T) {
	blob := []byte(`{"schema":` + itoa(Schema+1) + `,"seq":7,"room":{"id":"","name":"later","locked":false}}`)

	_, err := Unmarshal(blob)
	if !errors.Is(err, ErrSchema) {
		t.Fatalf("error = %v, want ErrSchema", err)
	}
}




func TestTheCurrentSchemaHasAGoldenSnapshot(t *testing.T) {
	name := filepath.Join("testdata", "snapshots", "schema-"+itoa(Schema)+".json")

	w := newWorld(t)
	w.spawn(Pawn{Name: "Goblin", X: 64, Y: 64, Visible: true})
	w.spawn(Pawn{Kind: PawnObject, Name: "Wagon", X: 200, Y: 200, Width: 128, Height: 256, Visible: true})

	
	
	
	
	
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
