package pages

import (
	"os"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/uievents"
)

func testScenesData() RoomScenesData {
	return RoomScenesData{
		RoomID: testTableRoomID,
		Scenes: []SceneCard{
			{
				RoomID:    testTableRoomID,
				ID:        "01BX5ZZKBKACTAV9WEVGEMMVS1",
				Name:      "The prepped ambush",
				PreviewID: "01BX5ZZKBKACTAV9WEVGEMMVS2",
				Updated:   Timestamp{ISO: "2026-09-12T09:00:00Z", Text: "12 Sep 2026, 09:00 UTC"},
			},
			{
				RoomID:  testTableRoomID,
				ID:      "01BX5ZZKBKACTAV9WEVGEMMVS3",
				Name:    "Town square",
				Updated: Timestamp{ISO: "2026-09-11T09:00:00Z", Text: "11 Sep 2026, 09:00 UTC"},
				Open:    true,
			},
		},
	}
}
func TestTheScenesWindowDrawsACardForEachScene(t *testing.T) {
	page := renderToString(t, RoomScenes(testScenesData()))
	for _, want := range []string{
		"The prepped ambush",
		"Town square",
		`src="/assets/images/01BX5ZZKBKACTAV9WEVGEMMVS2/preview"`,
		"12 Sep 2026, 09:00 UTC",
		"Save scene",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the shelf is missing %q:\n%s", want, page)
		}
	}
	if got := strings.Count(page, "/preview"); got != 1 {
		t.Errorf("%d pictures asked for, want 1; a scene whose map has gone must not ask for one", got)
	}
	if strings.Contains(page, roomScenesEmptyHeading) {
		t.Error("the empty state is drawn over a shelf that has scenes on it")
	}
}
func TestAnEmptyShelfSaysSoAndStillOffersTheSave(t *testing.T) {
	page := renderToString(t, RoomScenes(RoomScenesData{RoomID: testTableRoomID}))
	for _, want := range []string{roomScenesEmptyHeading, roomScenesEmptyBlurb, "Save scene"} {
		if !strings.Contains(page, want) {
			t.Errorf("an empty shelf is missing %q:\n%s", want, page)
		}
	}
}
func TestTheScenesWindowRefetchesOnALoadAndOnASave(t *testing.T) {
	page := renderToString(t, RoomScenes(testScenesData()))
	for _, want := range []string{uievents.Tabletop + " from:window", uievents.Scenes + " from:window"} {
		if !strings.Contains(page, want) {
			t.Errorf("the shelf never hears %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, `hx-trigger="load`) {
		t.Errorf("the shelf refetches itself the moment it is swapped in, which never stops:\n%s", page)
	}
	if !strings.Contains(page, `hx-get="`+ScenesWindowPath(testTableRoomID)+`"`) {
		t.Errorf("the shelf has nowhere to refetch from:\n%s", page)
	}
}
func TestTheScenesProseIsNotWrittenIntoTheTemplate(t *testing.T) {
	src, err := os.ReadFile("scenes.templ")
	if err != nil {
		t.Fatalf("scenes.templ is missing: %v", err)
	}
	for _, prose := range []string{roomScenesEmptyHeading, roomScenesEmptyBlurb} {
		if strings.Contains(string(src), prose) {
			t.Errorf("%q is written into scenes.templ; Tailwind reads .templ and a bare word "+
				"that happens to be a daisyUI class emits that whole component family", prose)
		}
	}
}
func TestTheSaveSceneFormIsAModalThatSaysHowToLeave(t *testing.T) {
	page := renderToString(t, RoomSceneSave(RoomSceneSaveData{RoomID: testTableRoomID}))
	for _, want := range []string{
		`hx-post="/rooms/` + testTableRoomID + `/scenes"`,
		`name="name"`,
		">Close</button>",
		"Save scene",
		uievents.ModalClose,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the save form is missing %q:\n%s", want, page)
		}
	}
	for _, banned := range []string{"btn-ghost", "btn-soft"} {
		if strings.Contains(page, banned) {
			t.Errorf("a button inside a dialog carries %q:\n%s", banned, page)
		}
	}
	if strings.Index(page, ">Close</button>") > strings.Index(page, ">Save scene</button>") {
		t.Error("Close comes after the affirmative action")
	}
}
func TestTheSaveSceneFormRoutesItsRefusalIntoItsOwnBlock(t *testing.T) {
	page := renderToString(t, RoomSceneSave(RoomSceneSaveData{RoomID: testTableRoomID}))
	block := "#errors-" + SceneSavePanel
	for _, want := range []string{
		`hx-target="` + block + `"`,
		`hx-status:422="target:` + block + `,swap:outerHTML"`,
		`id="errors-` + SceneSavePanel + `"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the save form is missing %s:\n%s", want, page)
		}
	}
}
func TestTheGMsTabletopMenuOpensTheScenesWindow(t *testing.T) {
	var found bool
	for _, item := range menuNamed(t, testRoomPage(room.RoleGM), "Tabletop").Items {
		if item.Label != "Scenes" {
			continue
		}
		found = true
		if item.Window.ID != ScenesWindow {
			t.Errorf("Scenes opens window %q, want %q", item.Window.ID, ScenesWindow)
		}
		if !strings.HasPrefix(item.Window.URL, "/fragment/room/scenes?room=") {
			t.Errorf("Scenes fetches %q", item.Window.URL)
		}
	}
	if !found {
		t.Error("the GM's Tabletop menu has no Scenes")
	}
	for _, item := range menuNamed(t, testRoomPage(room.RolePlayer), "Tabletop").Items {
		if item.Label == "Scenes" {
			t.Error("a player's Tabletop menu offers the GM's scenes")
		}
	}
}
func TestOpeningASceneIsGatedByTheConfirmModal(t *testing.T) {
	data := testScenesData()
	page := renderToString(t, RoomScenes(data))
	for _, want := range []string{
		`hx-post="/rooms/` + testTableRoomID + `/scenes/` + data.Scenes[0].ID + `/open"`,
		"hx-confirm=",
		`data-confirm-label="Open scene"`,
		"the turn order",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the open control is missing %s:\n%s", want, page)
		}
	}
	if got := strings.Count(page, "/open\""); got != 1 {
		t.Errorf("%d open buttons, want 1; the scene already on the tabletop must not offer to replace it with itself", got)
	}
	if !strings.Contains(page, "the fog, the drawing and the turn order") {
		t.Errorf("the confirm does not say what the load discards:\n%s", page)
	}
}
func TestSaveChangesIsOfferedOnTheOpenSceneAlone(t *testing.T) {
	data := testScenesData()
	page := renderToString(t, RoomScenes(data))
	if got := strings.Count(page, ">Save changes</button>"); got != 1 {
		t.Errorf("%d Save changes buttons, want 1 on the scene that is open", got)
	}
	if !strings.Contains(page, `hx-post="/rooms/`+testTableRoomID+`/scenes/`+data.Scenes[1].ID+`/save"`) {
		t.Errorf("Save changes is not on the open scene's card:\n%s", page)
	}
	if !strings.Contains(page, `data-confirm-label="Save changes"`) {
		t.Errorf("overwriting a scene is not confirmed:\n%s", page)
	}
}
func TestTheAutosaveToggleIsOnEveryCardAndReadsTheFlag(t *testing.T) {
	data := testScenesData()
	data.Scenes[0].Autosave = true
	page := renderToString(t, RoomScenes(data))
	if got := strings.Count(page, `name="autosave"`); got != 2 {
		t.Errorf("%d autosave toggles, want one per scene", got)
	}
	if got := strings.Count(page, "checked"); got != 1 {
		t.Errorf("%d toggles are on, want the one scene that autosaves", got)
	}
	if !strings.Contains(page, `hx-post="/scenes/`+data.Scenes[0].ID+`/autosave"`) {
		t.Errorf("the toggle does not post to the scene's own URL:\n%s", page)
	}
	if !strings.Contains(page, sceneAutosaveLabel) {
		t.Errorf("the toggle is not labelled %q:\n%s", sceneAutosaveLabel, page)
	}
}
func TestTheRoomBarNamesTheOpenSceneAndAsksOnce(t *testing.T) {
	page := renderToString(t, RoomSceneName(RoomSceneNameData{RoomID: testTableRoomID}))
	if !strings.Contains(page, "hx-trigger=\"load, "+uievents.Tabletop+" from:window, "+uievents.Scenes+" from:window\"") {
		t.Errorf("the page render never asks for the name:\n%s", page)
	}
	answer := renderToString(t, RoomSceneName(RoomSceneNameData{
		RoomID: testTableRoomID, Name: "The prepped ambush", Fetched: true,
	}))
	if strings.Contains(answer, "load") {
		t.Errorf("the answer arms itself again:\n%s", answer)
	}
	if !strings.Contains(answer, "The prepped ambush") {
		t.Errorf("the answer does not name the scene:\n%s", answer)
	}
}
func TestACardCarriesTheRestOfTheVerbs(t *testing.T) {
	data := testScenesData()
	card := data.Scenes[0]
	page := renderToString(t, RoomScenes(data))
	for _, want := range []string{
		`hx-patch="/scenes/` + card.ID + `/name"`,
		`hx-post="/scenes/` + card.ID + `/duplicate"`,
		`hx-delete="/scenes/` + card.ID + `"`,
		`data-confirm-label="Delete"`,
		`value="The prepped ambush"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the card is missing %s:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "hx-confirm=") {
		t.Errorf("deleting a scene is not confirmed:\n%s", page)
	}
	if got := strings.Count(page, "hx-delete="); got != 2 {
		t.Errorf("%d delete controls, want one per scene", got)
	}
}
