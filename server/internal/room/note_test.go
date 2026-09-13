package room

import (
	"strings"
	"testing"
)

func (w *world) writeNote(q, r int, title, body string) {
	w.t.Helper()
	w.apply(&NoteSet{Layer: w.layer, Q: q, R: r, Title: title, Body: body}, w.gm)
}
func TestANoteIsWrittenIntoACellAndStartsHidden(t *testing.T) {
	w := newWorld(t)
	w.writeNote(2, -1, "The standing stones", "Seven of them, one fallen.")
	note := w.s.Note(w.layer, 2, -1)
	if note == nil {
		t.Fatal("the cell holds no note")
	}
	if note.Title != "The standing stones" || note.Body != "Seven of them, one fallen." {
		t.Errorf("the note is %+v", *note)
	}
	if note.Revealed {
		t.Error("a fresh note is already shared with the players")
	}
	if len(w.s.Notes) != 1 {
		t.Errorf("the room holds %d notes, want 1", len(w.s.Notes))
	}
}
func TestWritingACellAgainReplacesTheWordsAndKeepsTheReveal(t *testing.T) {
	w := newWorld(t)
	w.writeNote(0, 0, "Ruined tower", "Empty.")
	w.apply(&NoteReveal{Layer: w.layer, Q: 0, R: 0, Revealed: true}, w.gm)
	w.writeNote(0, 0, "Ruined tower", "Owlbear inside.")
	note := w.s.Note(w.layer, 0, 0)
	if note == nil || note.Body != "Owlbear inside." {
		t.Fatalf("the note is %+v", note)
	}
	if !note.Revealed {
		t.Error("editing a shared note took it back from the players")
	}
	if len(w.s.Notes) != 1 {
		t.Errorf("the room holds %d notes, want the one cell", len(w.s.Notes))
	}
}
func TestTheSameCellOnTwoFloorsIsTwoNotes(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.writeNote(0, 0, "Above", "")
	w.apply(&NoteSet{Layer: cellar, Q: 0, R: 0, Title: "Below", Body: ""}, w.gm)
	if len(w.s.Notes) != 2 {
		t.Fatalf("the room holds %d notes, want one per floor", len(w.s.Notes))
	}
	if w.s.Note(cellar, 0, 0).Title != "Below" {
		t.Error("the two floors share one note")
	}
}
func TestRevealingAndRemovingANote(t *testing.T) {
	w := newWorld(t)
	w.writeNote(1, 1, "Camp", "Three goblins.")
	w.apply(&NoteReveal{Layer: w.layer, Q: 1, R: 1, Revealed: true}, w.gm)
	if !w.s.Note(w.layer, 1, 1).Revealed {
		t.Fatal("the note was not shared")
	}
	w.apply(&NoteReveal{Layer: w.layer, Q: 1, R: 1, Revealed: false}, w.gm)
	if w.s.Note(w.layer, 1, 1).Revealed {
		t.Fatal("the note was not taken back")
	}
	w.apply(&NoteRemove{Layer: w.layer, Q: 1, R: 1}, w.gm)
	if w.s.Note(w.layer, 1, 1) != nil {
		t.Error("the note is still in the cell")
	}
}
func TestRevealingOrRemovingACellWithNothingInItIsNotFound(t *testing.T) {
	w := newWorld(t)
	w.refuse(&NoteReveal{Layer: w.layer, Q: 5, R: 5, Revealed: true}, w.gm, CodeNotFound)
	w.refuse(&NoteRemove{Layer: w.layer, Q: 5, R: 5}, w.gm, CodeNotFound)
}
func TestANoteWithNothingInItIsRefused(t *testing.T) {
	w := newWorld(t)
	w.refuse(&NoteSet{Layer: w.layer, Q: 0, R: 0, Title: "  ", Body: "\n"}, w.gm, CodeInvalid)
	if len(w.s.Notes) != 0 {
		t.Error("an empty note put a marker on the map")
	}
	w.apply(&NoteSet{Layer: w.layer, Q: 0, R: 0, Body: "No title, but something to say."}, w.gm)
}
func TestANoteIsOnlySoLong(t *testing.T) {
	w := newWorld(t)
	w.refuse(&NoteSet{
		Layer: w.layer, Q: 0, R: 0, Title: "Fine", Body: strings.Repeat("a", NoteBodyLimit+1),
	}, w.gm, CodeInvalid)
	w.refuse(&NoteSet{
		Layer: w.layer, Q: 0, R: 0, Title: strings.Repeat("t", NameLimit+1), Body: "Fine",
	}, w.gm, CodeInvalid)
	w.apply(&NoteSet{Layer: w.layer, Q: 0, R: 0, Title: "Fine", Body: strings.Repeat("a", NoteBodyLimit)}, w.gm)
}
func TestTheHexKeyIsOnlySoBig(t *testing.T) {
	w := newWorld(t)
	body := strings.Repeat("a", NoteBodyLimit)
	for i := range NoteBytesBudget/NoteBodyLimit + 1 {
		if err := (&NoteSet{Layer: w.layer, Q: i, R: 0, Title: "", Body: body}).Authorize(w.s, w.gm); err != nil {
			t.Fatalf("note %d was refused by Authorize: %v", i, err)
		}
		_, err := (&NoteSet{Layer: w.layer, Q: i, R: 0, Title: "", Body: body}).Apply(w.s, w.gm, w.env)
		if err == nil {
			continue
		}
		e, ok := err.(*Error)
		if !ok || e.Code != CodeInvalid {
			t.Fatalf("note %d was refused with %v, want invalid", i, err)
		}
		if !strings.Contains(e.Message, "key") && !strings.Contains(e.Message, "notes") {
			t.Errorf("the refusal does not say the key is full: %q", e.Message)
		}
		return
	}
	t.Fatalf("the key took %d bytes of notes without refusing", NoteBytesBudget)
}
func TestACellOffTheEdgeOfTheWorldHoldsNoNote(t *testing.T) {
	w := newWorld(t)
	w.refuse(&NoteSet{Layer: w.layer, Q: CellLimit + 1, R: 0, Title: "Nowhere", Body: ""}, w.gm, CodeInvalid)
}
func TestAPlayerWritesOnAHexAndWhatTheyWriteIsShared(t *testing.T) {
	w := newWorld(t)
	w.apply(&NoteSet{Layer: w.layer, Q: 1, R: 0, Title: "Ford", Body: "Waist deep."}, w.pc)
	note := w.s.Note(w.layer, 1, 0)
	if note == nil || note.Title != "Ford" {
		t.Fatalf("the cell holds %+v, want the player's note", note)
	}
	if !note.Revealed {
		t.Error("a player wrote a note only the GM can read, so it vanished from under them")
	}
	if len(w.s.Project(RolePlayer).Notes) != 1 {
		t.Error("the player's own note is not in the players' copy")
	}
}
func TestAPlayerEditsASharedHexAndLeavesItShared(t *testing.T) {
	w := newWorld(t)
	w.writeNote(0, 0, "Ruined tower", "An owlbear.")
	w.apply(&NoteReveal{Layer: w.layer, Q: 0, R: 0, Revealed: true}, w.gm)
	w.apply(&NoteSet{Layer: w.layer, Q: 0, R: 0, Title: "Ruined tower", Body: "Owlbear dead."}, w.pc)
	note := w.s.Note(w.layer, 0, 0)
	if note == nil || note.Body != "Owlbear dead." {
		t.Fatalf("the cell holds %+v, want the player's edit", note)
	}
	if !note.Revealed {
		t.Error("a player editing a shared hex took it back from the table")
	}
}
func TestAPlayerCannotWriteOverAHexTheGMIsKeeping(t *testing.T) {
	w := newWorld(t)
	w.writeNote(0, 0, "Ruined tower", "An owlbear.")
	w.refuse(&NoteSet{Layer: w.layer, Q: 0, R: 0, Title: "Mine now", Body: ""}, w.pc, CodeForbidden)
	if note := w.s.Note(w.layer, 0, 0); note == nil || note.Title != "Ruined tower" {
		t.Fatalf("the GM's hidden note is %+v", note)
	}
}
func TestAPlayerWritesOnlyOnTheFloorTheTableIsShowing(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.refuse(&NoteSet{Layer: cellar, Q: 0, R: 0, Title: "Below", Body: ""}, w.pc, CodeForbidden)
}
func TestSharingAHexAndRubbingItOutStayTheGMs(t *testing.T) {
	w := newWorld(t)
	w.writeNote(0, 0, "Camp", "Three goblins.")
	w.apply(&NoteReveal{Layer: w.layer, Q: 0, R: 0, Revealed: true}, w.gm)
	w.refuse(&NoteReveal{Layer: w.layer, Q: 0, R: 0, Revealed: false}, w.pc, CodeForbidden)
	w.refuse(&NoteRemove{Layer: w.layer, Q: 0, R: 0}, w.pc, CodeForbidden)
}
func TestErasingACellRubsOutWhatIsWrittenOnItToo(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}, {Q: 1, R: 0}}}, w.gm)
	w.writeNote(0, 0, "Ruined tower", "An owlbear.")
	w.writeNote(1, 0, "Standing stones", "Seven.")
	w.apply(&TilesErase{Layer: w.layer, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	if w.s.Note(w.layer, 0, 0) != nil {
		t.Error("the erased cell kept its note")
	}
	if w.s.Note(w.layer, 1, 0) == nil {
		t.Error("erasing one cell took its neighbour's note")
	}
	w.apply(&TilesClear{Layer: w.layer}, w.gm)
	if len(w.s.Notes) != 0 {
		t.Errorf("%d notes survived a cleared floor", len(w.s.Notes))
	}
}
func TestAPlayerErasingTheirOwnTileLeavesTheWritingAlone(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.options(func(o *TableSetOptions) { o.PlayersCanStamp = true })
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.pc)
	w.writeNote(0, 0, "Ruined tower", "An owlbear.")
	w.apply(&TilesErase{Layer: w.layer, Cells: []Cell{{Q: 0, R: 0}}}, w.pc)
	if w.tileAt(w.layer, 0, 0) != nil {
		t.Fatal("the player could not erase their own tile")
	}
	if w.s.Note(w.layer, 0, 0) == nil {
		t.Error("a player rubbed out a note by erasing the tile under it")
	}
}
func TestDeletingAFloorTakesItsNotesWithIt(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	w.apply(&NoteSet{Layer: cellar, Q: 0, R: 0, Title: "Below", Body: ""}, w.gm)
	w.writeNote(0, 0, "Above", "")
	w.apply(&TableRemoveLayer{Layer: cellar}, w.gm)
	if len(w.s.Notes) != 1 || w.s.Notes[0].LayerID != w.layer {
		t.Fatalf("%d notes survived the floor, want the ground floor's alone", len(w.s.Notes))
	}
}
func TestClearingTheTabletopTakesTheHexKey(t *testing.T) {
	w := newWorld(t)
	w.writeNote(0, 0, "Camp", "Three goblins.")
	w.apply(&TableClear{}, w.gm)
	if len(w.s.Notes) != 0 {
		t.Error("the notes survived a cleared tabletop")
	}
}
