package pages

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)

func testPalette(n int, shelf int) RoomPaletteData {
	data := RoomPaletteData{RoomID: testTableRoomID}
	for i := range n {
		data.Entries = append(data.Entries, PaletteEntry{
			RoomID: testTableRoomID,
			ID:     "01ART" + string(rune('A'+i)),
			Name:   "Pines",
			Image:  "/assets/images/01ASSET",
		})
	}
	for i := range shelf {
		data.Shelf = append(data.Shelf, PaletteChoice{
			RoomID: testTableRoomID,
			ID:     "01ASSET" + string(rune('A'+i)),
			Name:   "Hills",
			Image:  "/assets/images/01ASSET",
			Held:   i == 0,
		})
	}
	return data
}
func TestThePaletteWindowRemovesAnEntryBehindAConfirmThatNamesTheCost(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(2, 0)))
	if got := strings.Count(page, "hx-delete="); got != 2 {
		t.Fatalf("%d remove controls, want one per entry:\n%s", got, page)
	}
	if !strings.Contains(page, `hx-delete="/rooms/`+testTableRoomID+`/palette/01ARTA"`) {
		t.Errorf("an entry does not address itself:\n%s", page)
	}
	prompt := strings.Contains(page, "hx-confirm=") && strings.Contains(page, "erased")
	if !prompt {
		t.Errorf("removing a picture does not say the stamped cells go with it:\n%s", page)
	}
}
func TestThePaletteWindowAddsFromTheTerrainShelf(t *testing.T) {
	data := testPalette(0, 2)
	data.Shelf[0].Held = false
	page := renderToString(t, RoomPalette(data))
	if got := strings.Count(page, `hx-post="/rooms/`+testTableRoomID+`/palette"`); got != 2 {
		t.Fatalf("%d shelf pictures post to the palette, want 2:\n%s", got, page)
	}
	if !strings.Contains(page, `{&#34;asset&#34;: &#34;01ASSETA&#34;}`) {
		t.Errorf("a shelf picture does not carry the asset it would add:\n%s", page)
	}
}
func TestAPictureAlreadyInTheBagIsMarkedAndCannotBeAddedTwice(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(1, 2)))
	if !strings.Contains(page, roomPaletteHeld) {
		t.Errorf("nothing says which shelf pictures are already in the bag:\n%s", page)
	}
	if got := strings.Count(page, `hx-post="/rooms/`+testTableRoomID+`/palette"`); got != 1 {
		t.Errorf("%d of the two shelf pictures can be added, want only the one that is not held:\n%s", got, page)
	}
}
func TestAFullPaletteSaysSoAndStopsOfferingMore(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(room.PaletteMax, 2)))
	if !strings.Contains(page, roomPaletteFull) {
		t.Errorf("a full palette does not say so:\n%s", page)
	}
	if strings.Contains(page, `hx-post="/rooms/`+testTableRoomID+`/palette"`) {
		t.Errorf("a full palette still offers to add another:\n%s", page)
	}
}
func TestEachPartIsTheSameMarkupWhetherItArrivesAloneOrInTheWindow(t *testing.T) {
	data := testPalette(1, 3)
	page := renderToString(t, RoomPalette(data))
	for name, part := range map[string]string{
		"the shelf": renderToString(t, RoomPaletteShelf(data)),
		"the bag":   renderToString(t, RoomPaletteBag(data)),
	} {
		if !strings.HasPrefix(strings.TrimSpace(part), "<div") {
			t.Fatalf("%s is not a bare element:\n%s", name, part)
		}
		if !strings.Contains(page, part) {
			t.Errorf("the window renders a second copy of %s:\n%s", name, part)
		}
	}
}
func TestAddingAPictureReplacesTwoShortListsAndNotTheWholeWindow(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(1, 2)))
	for _, want := range []string{`id="room-palette-bag"`, `id="room-palette-shelf"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the window has no %s to swap on its own:\n%s", want, page)
		}
	}
	if got := strings.Count(page, "hx-trigger="); got != 3 {
		t.Fatalf("%d things listen for a change, want the bag, the shelf and the search box", got)
	}
	outer := page[:strings.Index(page, `id="room-palette-bag"`)]
	if strings.Contains(outer, "hx-get=") {
		t.Errorf("the whole window still refetches itself, which is what loses the scroll:\n%s", outer)
	}
}
func TestTheShelfScrollsInsideTheWindowRatherThanTheWindowItself(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(2, 20)))
	shelf := renderToString(t, RoomPaletteShelf(testPalette(2, 20)))
	if !strings.Contains(shelf, "overflow-y-auto") {
		t.Errorf("the shelf does not scroll on its own:\n%s", shelf)
	}
	if !strings.Contains(page, "min-h-0") {
		t.Errorf("nothing lets the shelf shrink inside the window:\n%s", page)
	}
}
func TestThePaletteWindowKeepsWhatWasTypedWhenTheTableChanges(t *testing.T) {
	data := testPalette(0, 1)
	data.Query = "pine forest"
	page := renderToString(t, RoomPalette(data))
	if !strings.Contains(page, `value="pine forest"`) {
		t.Errorf("the search box forgot the term:\n%s", page)
	}
	if !strings.Contains(page, "q=pine+forest") {
		t.Errorf("a refetch would come back without the term:\n%s", page)
	}
}
func TestThePaletteWindowWritesNoMarkupOfItsOwnForAnEmptyBag(t *testing.T) {
	page := renderToString(t, RoomPalette(testPalette(0, 0)))
	for _, want := range []string{roomPaletteEmpty, roomPaletteShelfEmpty} {
		if !strings.Contains(page, want) {
			t.Errorf("the window is missing %q:\n%s", want, page)
		}
	}
}
