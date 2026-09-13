package pages

import (
	"strings"
	"testing"
)

const testNoteLayer = "01BX5ZZKBKACTAV9WEVGEMMVT7"

func testNoteData(isGM bool, exists bool) RoomNoteData {
	return RoomNoteData{
		RoomID:   testTableRoomID,
		Layer:    testNoteLayer,
		Q:        3,
		R:        -2,
		Title:    "Ruined tower",
		Body:     "An owlbear nests on the top floor.",
		Revealed: true,
		Exists:   exists,
		IsGM:     isGM,
	}
}
func TestTheGMsHexNoteIsAnEditorForTheCellItWasOpenedOn(t *testing.T) {
	page := renderToString(t, RoomNote(testNoteData(true, true)))
	for _, want := range []string{
		`name="title"`, `name="body"`, `name="revealed"`,
		`name="layer" value="` + testNoteLayer + `"`, `name="q" value="3"`, `name="r" value="-2"`,
		`hx-post="/rooms/` + testTableRoomID + `/notes"`,
		`hx-post="/rooms/` + testTableRoomID + `/notes/reveal"`,
		"Ruined tower", "An owlbear nests on the top floor.",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the editor is missing %s:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "hx-delete=") || !strings.Contains(page, "hx-confirm=") {
		t.Errorf("the GM cannot rub the note out, or can do it without being asked:\n%s", page)
	}
	if !strings.Contains(page, `hx-target="#`+noteActions+`"`) {
		t.Errorf("a save does not bring back the controls a first save unlocks:\n%s", page)
	}
	if !strings.Contains(page, `id="`+noteActions+`"`) {
		t.Errorf("the editor has nothing for a save to swap:\n%s", page)
	}
}
func TestAHexNobodyHasWrittenOnYetOffersNothingToShareOrDelete(t *testing.T) {
	data := testNoteData(true, false)
	data.Title, data.Body, data.Revealed = "", "", false
	page := renderToString(t, RoomNote(data))
	if strings.Contains(page, "hx-delete=") {
		t.Error("an empty hex offers to rub out a note that is not there")
	}
	if strings.Contains(page, `name="revealed"`) {
		t.Error("an empty hex offers to share nothing with the party")
	}
	if !strings.Contains(page, `id="`+noteActions+`"`) {
		t.Errorf("an empty hex has nothing for its first save to swap:\n%s", page)
	}
	if !strings.Contains(page, `name="body"`) {
		t.Error("an empty hex cannot be written on")
	}
}
func TestAPlayersHexNoteIsTheSameEditorWithoutTheGMsControls(t *testing.T) {
	page := renderToString(t, RoomNote(testNoteData(false, true)))
	for _, want := range []string{`name="title"`, `name="body"`, "Ruined tower", "An owlbear nests on the top floor."} {
		if !strings.Contains(page, want) {
			t.Errorf("a player cannot write the hex key: missing %s\n%s", want, page)
		}
	}
	for _, forbidden := range []string{`name="revealed"`, "hx-delete="} {
		if strings.Contains(page, forbidden) {
			t.Errorf("a player was handed %s:\n%s", forbidden, page)
		}
	}
}
func TestEverybodysWheelOffersAHexNote(t *testing.T) {
	for name, data := range map[string]RoomTableMenuData{
		"the GM":   NewTableMenu(testTableRoomID, true, nil),
		"a player": NewTableMenu(testTableRoomID, false, nil),
	} {
		page := renderToString(t, RoomTableMenu(data))
		if !strings.Contains(page, "data-table-menu-note") {
			t.Errorf("%s cannot reach the hex key from the wheel:\n%s", name, page)
		}
	}
	player := renderToString(t, RoomTableMenu(NewTableMenu(testTableRoomID, false, nil)))
	if strings.Contains(player, "data-table-menu-party") {
		t.Errorf("a player can move where the party starts:\n%s", player)
	}
}
func TestTheNoteActionsAppearOnlyOnceThereIsANote(t *testing.T) {
	held := renderToString(t, RoomNoteActions(testNoteData(true, true)))
	if !strings.Contains(held, `name="revealed"`) || !strings.Contains(held, "hx-delete=") {
		t.Errorf("a written hex cannot be shared or rubbed out:\n%s", held)
	}
	empty := testNoteData(true, false)
	empty.Title, empty.Body, empty.Revealed = "", "", false
	blank := renderToString(t, RoomNoteActions(empty))
	if strings.Contains(blank, `name="revealed"`) || strings.Contains(blank, "hx-delete=") {
		t.Errorf("an empty hex offers controls for a note that is not there:\n%s", blank)
	}
	if !strings.Contains(blank, `id="`+noteActions+`"`) {
		t.Errorf("the empty block is not a target for the first save:\n%s", blank)
	}
}
