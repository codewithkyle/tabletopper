package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/room"
)

const (
	testViewRoomID      = "01BX5ZZKBKACTAV9WEVGEMMVT0"
	testViewCharacterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	testViewPawnID      = "01BX5ZZKBKACTAV9WEVGEMMVT7"
)

func testCharacterView(section string) CharacterViewData {
	return CharacterViewData{
		RoomID:      testViewRoomID,
		CharacterID: testViewCharacterID,
		PawnID:      testViewPawnID,
		Section:     section,
		Level:       SpellsPrepared,
		Sheet:       testSharedSheet(),
		Levels: []SpellLevel{
			{Level: 0, Slots: "0", Used: "0", Count: 2},
			{Level: 1, Slots: "4", Used: "1", Count: 3},
			{Level: 2, Slots: "0", Used: "0", Count: 0},
		},
		Current: SpellLevel{Level: 1, Slots: "4", Used: "1", Count: 2},
		LevelSpells: []SharedSpell{
			{Name: "Hunter's Mark", School: "Divination", CastingTime: "Bonus action", Range: "90 feet", Duration: "1 hour", Components: "V", Description: "You choose a creature you can see.", Prepared: true},
			{Name: "Ensnaring Strike", School: "Conjuration", CastingTime: "Bonus action"},
		},
	}
}
func testSpellLevelView() CharacterViewData {
	data := testCharacterView(SheetSectionSpells)
	data.Level = 1
	return data
}
func attr(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "&", "&amp;"), "'", "&#39;")
}
func TestTheCharacterViewIsReadOnly(t *testing.T) {
	views := map[string]CharacterViewData{"a spell level": testSpellLevelView()}
	for _, tab := range SheetTabs() {
		views[tab.Section] = testCharacterView(tab.Section)
	}
	for name, data := range views {
		body := renderString(t, CharacterView(data))
		for _, forbidden := range []string{"<form", "<input", "<select", "<textarea", "hx-post", "hx-delete", "/characters/", "data-modal-open"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s in a read-only sheet carries %q:\n%s", name, forbidden, body)
			}
		}
	}
}
func TestTheCharacterViewSwapsItsOwnBodyAndNoOtherWindows(t *testing.T) {
	data := testCharacterView(SheetSectionSkills)
	body := renderString(t, CharacterView(data))
	for _, tab := range SheetTabs() {
		if want := `hx-get="` + attr(data.SectionPath(tab.Section)) + `"`; !strings.Contains(body, want) {
			t.Errorf("the view offers no way to %s: no %s", tab.Section, want)
		}
	}
	if got := strings.Count(body, `hx-target="#`+data.BodyID()+`"`); got != len(SheetTabs()) {
		t.Errorf("%d tabs swap the view's own body, want %d", got, len(SheetTabs()))
	}
	if strings.Contains(body, `hx-target="#`+SheetBodyID+`"`) {
		t.Error("a tab in the GM's view swaps the player's own sheet window")
	}
	other := testCharacterView(SheetSectionSkills)
	other.CharacterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"
	if data.BodyID() == other.BodyID() {
		t.Error("two characters' sheets open at once share a body id")
	}
}
func TestTheCharacterViewSaysWhereItIs(t *testing.T) {
	views := []CharacterViewData{testSpellLevelView()}
	for _, tab := range SheetTabs() {
		views = append(views, testCharacterView(tab.Section))
	}
	if want := "&section=spells&level=1"; !strings.HasSuffix(testSpellLevelView().Path(), want) {
		t.Errorf("a spell level says it is at %q, want it to end %q", testSpellLevelView().Path(), want)
	}
	for _, data := range views {
		body := renderString(t, CharacterView(data))
		root := regexp.MustCompile(`^<div id="` + data.BodyID() + `"[^>]*>`).FindString(body)
		if want := `data-window-path="` + attr(data.Path()) + `"`; !strings.Contains(root, want) {
			t.Errorf("the %s tab's root %q does not carry %s", data.Section, root, want)
		}
		if got := strings.Count(body, "data-window-path"); got != 1 {
			t.Errorf("the %s tab names %d places, want only its root", data.Section, got)
		}
	}
}
func TestTheCharacterViewOpensOnTheCharacterTab(t *testing.T) {
	if got, want := CharacterViewPath(testViewRoomID, testViewCharacterID), "/fragment/room/character-sheet?room="+testViewRoomID+"&character="+testViewCharacterID; got != want {
		t.Errorf("CharacterViewPath = %q, want %q", got, want)
	}
	if !strings.HasPrefix(CharacterViewPath(testViewRoomID, testViewCharacterID), "/fragment/") {
		t.Error("the GM's sheet window would be refused: its url is not a fragment")
	}
}
func TestEachViewTabHoldsItsOwnPanels(t *testing.T) {
	for section, want := range map[string][]string{
		SheetSectionMain:      {"Vitals", "Core Stats", "Features &amp; Traits", "Proficiencies &amp; Training", "Identity", "Personality", "Appearance"},
		SheetSectionAbilities: {"Abilities", "Saving Throws"},
		SheetSectionSkills:    {"Skills", "Passive Perception"},
		SheetSectionAttacks:   {"Attacks", "Longbow"},
		SheetSectionInventory: {"Inventory", "Studded Leather", "Hempen Rope"},
		SheetSectionSpells:    {"Prepared Spells", "Hunter&#39;s Mark", "Spell Slots"},
	} {
		body := renderString(t, characterViewSection(testCharacterView(section)))
		last := -1
		for _, heading := range want {
			at := strings.Index(body, heading)
			if at < 0 {
				t.Errorf("the %s tab does not show %q:\n%s", section, heading, body)
				continue
			}
			if at < last {
				t.Errorf("the %s tab shows %q out of order", section, heading)
			}
			last = at
		}
	}
}
func TestTheInventoryTabShowsEveryColumnTheEditorDoes(t *testing.T) {
	data := testCharacterView(SheetSectionInventory)
	body := renderString(t, characterViewSection(data))
	for _, want := range []string{
		">Item<", ">Eq<", ">Qty<", ">Value<", ">Weight<",
		"Studded Leather", "45 gp", ">13<",
		"Hempen Rope", ">2<", "1 gp", ">10<",
		`aria-label="Equipped"`,
		"Fifty feet, knotted every ten.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the inventory tab does not show %s:\n%s", want, body)
		}
	}
	if got := strings.Count(body, `aria-label="Equipped"`); got != 1 {
		t.Errorf("%d items marked equipped, want only the leather", got)
	}
	if got := strings.Count(body, "<details"); got != 1 {
		t.Errorf("%d details accordions, want one for the one item with details", got)
	}
	if strings.Contains(body, "<details open") || strings.Contains(body, " open>") {
		t.Errorf("an item's details start open:\n%s", body)
	}
}
func TestEveryLineUnderAReadOnlyItemStartsInItsFirstColumn(t *testing.T) {
	row := renderString(t, inventoryReadingRow("group", SharedItem{Name: "Rope", Description: "Long."}))
	classes := regexp.MustCompile(`class="([^"]*\brow-start-[2-9][^"]*)"`).FindAllStringSubmatch(row, -1)
	if len(classes) == 0 {
		t.Fatal("found nothing below the row, so this proves nothing")
	}
	for _, class := range classes {
		if !strings.Contains(class[1], "col-start-1") {
			t.Errorf("%q starts wherever the grid cursor is", class[1])
		}
	}
}
func TestTheSpellsTabReachesEveryLevelInsideTheView(t *testing.T) {
	for _, data := range []CharacterViewData{testCharacterView(SheetSectionSpells), testSpellLevelView()} {
		body := renderString(t, CharacterView(data))
		for _, level := range spellLevelChoices() {
			want := `hx-get="` + attr(data.LevelPath(level)) + `" hx-target="#` + data.BodyID() + `"`
			if !strings.Contains(body, want) {
				t.Errorf("level %d is not reachable from level %d of the view", level, data.Level)
			}
		}
		if strings.Contains(body, `hx-target="#`+SheetBodyID+`"`) {
			t.Error("a level in the GM's view swaps the player's own sheet window")
		}
	}
	prepared := renderString(t, slotsReading(testCharacterView(SheetSectionSpells)))
	for _, level := range []int{0, 1} {
		button := `<button type="button" class="` + spellLevelLink + `" hx-get="` + attr(testCharacterView(SheetSectionSpells).LevelPath(level)) + `"`
		if !strings.Contains(prepared, button) {
			t.Errorf("the slot overview does not open level %d:\n%s", level, prepared)
		}
	}
	if strings.Contains(prepared, SpellLevelName(2)) {
		t.Error("the slot overview lists a level with no spells and no slots")
	}
	if !strings.Contains(prepared, "3 of 4 slots left") {
		t.Errorf("the slot overview does not say how many slots are left:\n%s", prepared)
	}
}
func TestALevelShowsEverySpellWithItsDetailsClosed(t *testing.T) {
	body := renderString(t, characterViewSection(testSpellLevelView()))
	for _, want := range []string{
		"Level 1", "3 of 4 slots left",
		"Hunter&#39;s Mark", "Ensnaring Strike", "Bonus action",
		"Divination", "90 feet", "1 hour", "You choose a creature you can see.",
		"Conjuration",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the level does not show %q:\n%s", want, body)
		}
	}
	if got := strings.Count(body, `badge-sm ms-auto">Prepared<`); got != 1 {
		t.Errorf("%d spells marked prepared, want only Hunter's Mark", got)
	}
	if got := strings.Count(body, "<details"); got != 2 {
		t.Errorf("%d details accordions, want one per spell", got)
	}
	if strings.Contains(body, " open>") || strings.Contains(body, "<details open") {
		t.Errorf("a spell's details start open:\n%s", body)
	}
	if !strings.Contains(body, `name="`+testSpellLevelView().DetailsGroup("spell")+`"`) {
		t.Error("the spells' details are not one accordion")
	}
	other := testSpellLevelView()
	other.CharacterID = "01BX5ZZKBKACTAV9WEVGEMMVT9"
	if other.DetailsGroup("spell") == testSpellLevelView().DetailsGroup("spell") || testSpellLevelView().DetailsGroup("spell") == "spell-details" {
		t.Error("opening a spell in one sheet window closes one in another")
	}
}
func TestSlotsLeftCountsDownToNothing(t *testing.T) {
	for _, c := range []struct {
		level SpellLevel
		want  string
	}{
		{SpellLevel{Level: 0, Slots: "0", Used: "0"}, "Unlimited"},
		{SpellLevel{Level: 1, Slots: "4", Used: "1"}, "3 of 4 slots left"},
		{SpellLevel{Level: 3, Slots: "1", Used: "0"}, "1 of 1 slot left"},
		{SpellLevel{Level: 3, Slots: "2", Used: "5"}, "0 of 2 slots left"},
		{SpellLevel{Level: 5, Slots: "0", Used: "0"}, "No slots"},
	} {
		if got := SpellSlotsLeft(c.level); got != c.want {
			t.Errorf("SpellSlotsLeft(%+v) = %q, want %q", c.level, got, c.want)
		}
	}
}
func TestAnEmptyViewTabSaysSo(t *testing.T) {
	for section, want := range map[string]string{
		SheetSectionAttacks:   "No attacks on this sheet.",
		SheetSectionInventory: "Nothing in this inventory.",
		SheetSectionSpells:    "Nothing is prepared.",
		"slots":               "No spell levels in use.",
		"level":               "No spells at this level.",
	} {
		data := testCharacterView(section)
		switch section {
		case "slots":
			data = testCharacterView(SheetSectionSpells)
		case "level":
			data = testSpellLevelView()
		}
		empty := data
		empty.Sheet = SharedCharacterSheet{}
		empty.Levels = nil
		empty.LevelSpells = nil
		if body := renderString(t, characterViewSection(empty)); !strings.Contains(body, want) {
			t.Errorf("an empty %s tab does not say %q:\n%s", section, want, body)
		}
		if body := renderString(t, characterViewSection(data)); strings.Contains(body, want) {
			t.Errorf("a full %s tab still says %q", section, want)
		}
	}
}
func TestTheViewsVitalsFollowThePawn(t *testing.T) {
	data := testCharacterView(SheetSectionMain)
	body := renderString(t, characterViewSection(data))
	for _, section := range LiveSheetSections() {
		want := `hx-get="` + attr(data.SectionPath(section)) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("the %s panel does not refetch itself: no %s", section, want)
		}
	}
	if got := strings.Count(body, `hx-trigger="`+attr(data.Trigger())+`"`); got != len(LiveSheetSections()) {
		t.Errorf("%d panels listen for the pawn, want %d", got, len(LiveSheetSections()))
	}
	if !strings.Contains(data.Trigger(), "'"+testViewPawnID+"'") {
		t.Errorf("the refetch %q does not filter on the character's own pawn", data.Trigger())
	}
	data.PawnID = ""
	if body := renderString(t, characterViewSection(data)); strings.Contains(body, "hx-trigger") {
		t.Errorf("a character with no pawn on the table listens for one:\n%s", body)
	}
}
func TestALiveViewPanelRedrawsItselfAlone(t *testing.T) {
	for _, section := range LiveSheetSections() {
		data := testCharacterView(section)
		body := renderString(t, CharacterViewSection(data))
		if !strings.HasPrefix(body, `<div hx-get="`+attr(data.SectionPath(section))+`"`) {
			t.Errorf("the %s panel does not come back wrapped in its own refetch:\n%s", section, body)
		}
		if strings.Contains(body, "data-window-path") || strings.Contains(body, "aria-label=\"Character sheet sections\"") {
			t.Errorf("the %s panel came back with the whole window:\n%s", section, body)
		}
	}
}
func TestTheCharacterSheetButtonOnAPawnIsTheGMsAlone(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.Character = true
	data.Pawn.Name = "Ilyana"
	data.Pawn.CharacterID = testViewCharacterID
	body := html(t, RoomPawnFragment(data))
	for _, want := range []string{
		`data-window="` + CharacterViewWindow(testViewCharacterID) + `"`,
		`data-window-url="` + attr(CharacterViewPath(testPawnRoomID, testViewCharacterID)) + `"`,
		`data-window-title="Ilyana"`,
		`data-tip="Character sheet"`,
		`aria-label="Character sheet for Ilyana"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the GM's panel on a character has no %s:\n%s", want, body)
		}
	}
	data.IsGM = false
	if body := html(t, RoomPawnFragment(data)); strings.Contains(body, "data-window=") {
		t.Errorf("a player was offered somebody's sheet:\n%s", body)
	}
	data.IsGM = true
	data.Pawn.CharacterID = ""
	if body := html(t, RoomPawnFragment(data)); strings.Contains(body, "Character sheet") {
		t.Errorf("a pawn with no character offers a sheet:\n%s", body)
	}
}
func TestTheGMOpensEachPlayersSheetFromThePlayerList(t *testing.T) {
	data := membersFor(true)
	player := data.Members[1]
	player.CharacterID = testViewCharacterID
	data.Members[1] = player
	page := renderToString(t, RoomMembers(data))
	if want := `data-window="` + CharacterViewWindow(testViewCharacterID) + `"`; strings.Count(page, want) != 1 {
		t.Errorf("the player list does not open %s's sheet exactly once:\n%s", player.Name, page)
	}
	if !strings.Contains(page, `data-window-url="`+attr(CharacterViewPath(testRoomIDText, testViewCharacterID))+`"`) {
		t.Errorf("the player list opens the wrong sheet:\n%s", page)
	}
	if got := strings.Count(page, "data-window="); got != 1 {
		t.Errorf("%d sheet buttons for one player with a character, want 1", got)
	}
	data.GM = false
	if page := renderToString(t, RoomMembers(data)); strings.Contains(page, "data-window=") {
		t.Errorf("a player was offered somebody's sheet from the player list:\n%s", page)
	}
}
func TestTheGMsSheetWindowIsTheSizeOfThePlayersOwn(t *testing.T) {
	data := testPawnPanel()
	data.Pawn.CharacterID = testViewCharacterID
	body := html(t, RoomPawnFragment(data))
	for _, want := range []string{`data-window-width="520"`, `data-window-height="640"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the sheet button has no %s", want)
		}
	}
	page := testRoomPage(room.RolePlayer)
	page.CharacterID = testViewCharacterID
	sized := false
	for _, item := range page.characterMenu().Items {
		if item.Window.ID != SheetWindow {
			continue
		}
		sized = item.Window.Width == SheetWindowWidth && item.Window.Height == SheetWindowHeight
	}
	if !sized {
		t.Error("the player's own sheet window is not the size the GM's opens at")
	}
}
func TestTheClientRetitlesTheGMsSheetWindowsByTheSameName(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "js", "room", "sheet-window.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `CHARACTER_WINDOW = "` + CharacterViewWindowPrefix + `"`; !strings.Contains(string(source), want) {
		t.Errorf("sheet-window.ts does not declare %s", want)
	}
}
