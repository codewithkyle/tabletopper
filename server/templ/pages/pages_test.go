package pages

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/a-h/templ"
	"github.com/oklog/ulid/v2"
)

const closingForms = 4

func TestPagesRenderConcurrently(t *testing.T) {
	pages := map[string]func() error{
		"homepage":               func() error { return render(Homepage(session.UserSession{})) },
		"characters":             func() error { return render(Characters([]queries.Character{})) },
		"new-character-fragment": func() error { return render(NewCharacterFragment()) },
		"edit-character": func() error {
			return render(EditCharacter(EditCharacterPageData{}))
		},
		"edit-character-spell-level": func() error {
			return render(EditCharacterSpellLevel(SpellLevelPageData{Level: 3, Current: SpellLevel{Level: 3, Slots: "4", Used: "1"}}))
		},
		"edit-character-inventory": func() error { return render(EditCharacterInventory(InventoryPageData{})) },
		"edit-character-journal": func() error {
			return render(EditCharacterJournal(JournalPageData{Entries: []JournalEntry{testJournalEntry()}}))
		},
		"edit-character-journal-entry": func() error {
			return render(EditCharacterJournalEntry(JournalEntryPageData{}))
		},
		"character-sheet-window": func() error {
			return render(CharacterSheetWindow(SheetWindowData{Section: SheetSectionMain}))
		},
		"journal-link-fragment": func() error { return render(JournalLinkFragment()) },
		"journal-entries-fragment": func() error {
			return render(JournalEntriesFragment(JournalPageData{Entries: []JournalEntry{testJournalEntry()}}))
		},
		"share-dialog": func() error {
			return render(ShareDialog(ShareDialogData{}))
		},
		"shared-journal-entry": func() error {
			return render(SharedJournalEntry(SharedJournalData{}))
		},
		"shared-character-page": func() error {
			return render(SharedCharacterPage(SharedCharacterSheet{}))
		},
		"share-locked":      func() error { return render(ShareLocked(ShareLockedData{})) },
		"share-unavailable": func() error { return render(ShareUnavailable()) },
		"account-settings-fragment": func() error {
			return render(AccountSettingsFragment(testAccountSettings()))
		},
		"account-welcome-fragment": func() error {
			return render(AccountWelcomeFragment(testAccountSettings()))
		},
		"monsters": func() error {
			return render(Monsters(MonsterListData{Monsters: []MonsterSummary{testMonsterCard()}}))
		},
		"monsters-empty":         func() error { return render(Monsters(MonsterListData{})) },
		"monster-cards-fragment": func() error { return render(MonsterCardsFragment(MonsterListData{Query: "goblin"})) },
		"new-monster-fragment":   func() error { return render(NewMonsterFragment()) },
		"edit-monster": func() error {
			return render(EditMonster(EditMonsterPageData{}))
		},
		"stat-block-fragment": func() error {
			return render(MonsterStatBlockFragment(testStatBlock()))
		},
		"stat-block-panel": func() error {
			return render(MonsterStatBlockPanel(testStatBlock()))
		},
		"assets":         func() error { return render(MapAssets([]MapAsset{testMapCard()})) },
		"assets-empty":   func() error { return render(MapAssets(nil)) },
		"assets-tokens":  func() error { return render(TokenAssets(nil)) },
		"assets-avatars": func() error { return render(AvatarAssets(nil)) },
		"assets-tokens-full": func() error {
			return render(TokenAssets([]LibraryAsset{testLibraryCard("tokens")}))
		},
		"assets-avatars-full": func() error {
			return render(AvatarAssets([]LibraryAsset{testLibraryCard("avatars")}))
		},
		"assets-music": func() error { return render(MusicAssets(nil)) },
		"assets-music-full": func() error {
			return render(MusicAssets([]MusicTrack{testMusicTrack()}))
		},
		"assets-maps-searched":    func() error { return render(MapCards(nil, "keep")) },
		"assets-tokens-searched":  func() error { return render(TokenCards(nil, "wagon")) },
		"assets-avatars-searched": func() error { return render(AvatarCards(nil, "elf")) },
		"assets-music-searched":   func() error { return render(MusicCards(nil, "rain")) },
		"rooms": func() error {
			return render(Rooms(RoomsPageData{Rooms: []RoomSummary{{ID: "01BX5ZZKBKACTAV9WEVGEMMVT0", Name: "Curse of Strahd", Code: "AB2C"}}}))
		},
		"rooms-empty":         func() error { return render(Rooms(RoomsPageData{})) },
		"new-room-fragment":   func() error { return render(NewRoomFragment()) },
		"join-room":           func() error { return render(JoinRoom(JoinRoomPageData{})) },
		"join-room-prefilled": func() error { return render(JoinRoom(JoinRoomPageData{Code: "AB2C"})) },
		"room-gm":             func() error { return render(Room(testRoomPage(room.RoleGM))) },
		"room-player":         func() error { return render(Room(testRoomPage(room.RolePlayer))) },
		"room-closed": func() error {
			data := testRoomPage(room.RoleGM)
			data.Closed = true
			data.Code = ""
			return render(Room(data))
		},
		"room-lock-item": func() error { return render(RoomLockItem(testRoomPage(room.RoleGM))) },
		"sign-in":        func() error { return render(SignIn(ClerkFrontend{})) },
		"tos":            func() error { return render(TOS()) },
	}
	var wg sync.WaitGroup
	for name, page := range pages {
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := page(); err != nil {
					t.Errorf("%s: %v", name, err)
				}
			}()
		}
	}
	wg.Wait()
}
func render(c templ.Component) error {
	var buf bytes.Buffer
	return c.Render(context.Background(), &buf)
}
func TestEditCharacterRendersOneFormPerPanel(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	base := "/characters/" + id
	panels := map[string]string{
		"identity":      base + "/identity",
		"abilities":     base + "/abilities",
		"core-stats":    base + "/core-stats",
		"vitals":        base + "/vitals",
		"proficiencies": base + "/proficiencies",
		"saving_throws": base + "/bonuses/saving_throws",
		"skills":        base + "/bonuses/skills",
		"features":      base + "/features",
		"personality":   base + "/personality",
		"appearance":    base + "/appearance",
	}
	var buf bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: id}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for panel, action := range panels {
		if want := `hx-post="` + action + `"`; !strings.Contains(markup, want) {
			t.Errorf("no panel posts to %s", action)
		}
		if want := `id="errors-` + panel + `"`; !strings.Contains(markup, want) {
			t.Errorf("panel %q has no error block to swap into", panel)
		}
	}
	for _, absent := range []string{base + "/spells", base + "/spells/slots/1"} {
		if strings.Contains(markup, `hx-post="`+absent+`"`) {
			t.Errorf("the Character tab carries %s, which belongs to the spells pages", absent)
		}
	}
	if got := strings.Count(markup, "hx-post="); got != len(panels)+1 {
		t.Errorf("posting elements = %d, want %d (one per panel, plus Add Attack)", got, len(panels)+1)
	}
	if got := strings.Count(markup, "<form"); got != len(panels)+closingForms {
		t.Errorf("forms = %d, want %d", got, len(panels)+closingForms)
	}
	if got := strings.Count(markup, `hx-trigger="input delay:1s, repeater:changed"`); got != len(panels) {
		t.Errorf("debounced panels = %d, want %d", got, len(panels))
	}
	if strings.Contains(markup, `type="submit"`) {
		t.Error("the editor still renders a submit button")
	}
	assertCharacterTabs(t, markup, base+"/edit")
}
func assertCharacterTabs(t *testing.T, markup string, current string) {
	t.Helper()
	base := strings.TrimSuffix(current, "/edit")
	base = strings.SplitN(base, "/edit/", 2)[0]
	for _, href := range []string{base + "/edit", base + "/edit/inventory", base + "/edit/spells/0", base + "/edit/journal"} {
		if !strings.Contains(markup, `href="`+href+`"`) {
			t.Errorf("no way to reach %s from here", href)
		}
	}
	if want := `href="` + current + `" aria-current="page"`; !strings.Contains(markup, want) {
		t.Errorf("the current tab is not %s", current)
	}
}
func testSpellCounters(level int) SpellLevel {
	return SpellLevel{Level: level, Slots: "0", Used: "0"}
}
func TestNewCharacterFragmentIsOneQuestion(t *testing.T) {
	var buf bytes.Buffer
	if err := NewCharacterFragment().Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`hx-post="/characters"`,
		`hx-target="#errors-new-character"`,
		`hx-status:422="target:#errors-new-character,swap:outerHTML"`,
		`id="errors-new-character"`,
		`name="name"`,
		"required",
		`type="submit"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment is missing %s\n%s", want, body)
		}
	}
	if forms := strings.Count(body, "<form"); forms != 1 {
		t.Errorf("fragment has %d forms, want 1", forms)
	}
	if !strings.Contains(body, "modal:close") {
		t.Errorf("fragment has no Close button\n%s", body)
	}
	if strings.Contains(body, "placeholder=") {
		t.Errorf("the name field has a placeholder\n%s", body)
	}
	if inputs := strings.Count(body, "<input"); inputs != 1 {
		t.Errorf("fragment has %d inputs, want 1", inputs)
	}
}

const (
	testItemID    = "01BX5ZZKBKACTAV9WEVGEMMVS0"
	testItemPanel = "errors-inventory-" + testItemID
)

func TestInventoryRowIsItsOwnForm(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	base := "/characters/" + characterID + "/inventory/" + testItemID
	var buf bytes.Buffer
	item := InventoryItem{ID: testItemID, Name: "Longsword", Quantity: "2", Weight: "3", Value: "15 gp"}
	if err := InventoryRow(characterID, item).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`hx-post="` + base + `"`,
		`hx-trigger="input delay:1s"`,
		`hx-target="#` + testItemPanel + `"`,
		`hx-status:422="target:#` + testItemPanel + `,swap:outerHTML"`,
		`id="` + testItemPanel + `"`,
		`hx-delete="` + base + `"`,
		`hx-swap="delete"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("row is missing %s\n%s", want, body)
		}
	}
	if forms := strings.Count(body, "<form"); forms != 1 {
		t.Errorf("row has %d forms, want 1 -- the row IS the form", forms)
	}
	if strings.Contains(body, `type="submit"`) {
		t.Error("the row renders a submit button")
	}
}
func TestInventoryRowAlwaysRendersEveryControl(t *testing.T) {
	for _, item := range []InventoryItem{
		{ID: testItemID},
		{ID: testItemID, Name: "Longsword", Quantity: "2", Weight: "3", Value: "15 gp", Equipped: true, Description: "1d8"},
	} {
		var buf bytes.Buffer
		if err := InventoryRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", item).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		body := buf.String()
		for _, name := range []string{"name", "quantity", "value", "weight", "equipped", "description"} {
			if !strings.Contains(body, `name="`+name+`"`) {
				t.Errorf("equipped=%v: no control named %q\n%s", item.Equipped, name, body)
			}
		}
		if !strings.Contains(body, `type="checkbox"`) {
			t.Errorf("equipped=%v: the equipped control is not a checkbox", item.Equipped)
		}
		if checked := strings.Contains(body, "checked"); checked != item.Equipped {
			t.Errorf("equipped=%v but checked=%v", item.Equipped, checked)
		}
	}
}
func TestInventoryPageIsOneFormPerItem(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	data := InventoryPageData{
		CharacterID: characterID,
		Items: []InventoryItem{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Longsword"},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Name: "Rope"},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS2", Name: "Rations"},
		},
	}
	if err := EditCharacterInventory(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if got := strings.Count(body, "hx-post=\"/characters/"+characterID+"/inventory/"); got != len(data.Items) {
		t.Errorf("saving rows = %d, want %d", got, len(data.Items))
	}
	if got := strings.Count(body, "<form"); got != len(data.Items)+closingForms {
		t.Errorf("forms = %d, want %d", got, len(data.Items)+closingForms)
	}
	if want := `hx-post="/characters/` + characterID + `/inventory"`; !strings.Contains(body, want) {
		t.Errorf("no add button posting to %s", want)
	}
	if !strings.Contains(body, `hx-swap="append"`) {
		t.Error("the add button does not append its reply")
	}
}
func TestEquippedItemsIsAViewAndNotAForm(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	items := []InventoryItem{
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Longsword", Quantity: "1", Description: "1d8 slashing"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Name: "Javelin", Quantity: "4"},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS2", Name: "", Quantity: "1"},
	}
	if err := equippedItems(characterID, items).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, forbidden := range []string{"<form", "<input", "<textarea", "hx-post", "hx-delete"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the equipped view carries %s\n%s", forbidden, body)
		}
	}
	for _, want := range []string{"Longsword", "1d8 slashing", "Javelin"} {
		if !strings.Contains(body, want) {
			t.Errorf("the equipped view is missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "&#215; 1<") {
		t.Error("a quantity of 1 is printed beside an item")
	}
	if !strings.Contains(body, "&#215; 4") {
		t.Errorf("a quantity above 1 is not printed\n%s", body)
	}
	if !strings.Contains(body, "Unnamed item") {
		t.Errorf("an unnamed equipped row renders as nothing\n%s", body)
	}
}
func TestEquippedItemsEmptyStatePointsAtTheInventoryPage(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	if err := equippedItems(characterID, nil).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if want := `href="/characters/` + characterID + `/edit/inventory"`; !strings.Contains(body, want) {
		t.Errorf("the empty state does not point at %s\n%s", want, body)
	}
	if !strings.Contains(body, "Equipped") {
		t.Errorf("the empty state does not name the control that fills it\n%s", body)
	}
}
func TestCharacterPageHasNoWeaponsOrResourcesPanel(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	data := EditCharacterPageData{
		CharacterID: id,
		Equipped:    []InventoryItem{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Chain Mail", Quantity: "1"}},
	}
	if err := EditCharacter(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, gone := range []string{"/rows/weapons", "/rows/resources", "weapons-name", "resources-name"} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still carries %s", gone)
		}
	}
	if !strings.Contains(body, "Chain Mail") {
		t.Errorf("the equipped rows are not rendered on the page\n%s", body)
	}
}

const (
	testSpellID    = "01BX5ZZKBKACTAV9WEVGEMMVS0"
	testSpellPanel = "errors-spell-" + testSpellID
)

func TestSpellRowIsItsOwnForm(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	base := "/characters/" + characterID + "/spells/3/" + testSpellID
	var buf bytes.Buffer
	spell := Spell{ID: testSpellID, Level: 3, Name: "Fireball", School: "Evocation", CastingTime: "Action"}
	if err := SpellRow(characterID, spell).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		`hx-post="` + base + `"`,
		`hx-trigger="input delay:1s"`,
		`hx-target="#` + testSpellPanel + `"`,
		`hx-status:422="target:#` + testSpellPanel + `,swap:outerHTML"`,
		`id="` + testSpellPanel + `"`,
		`hx-delete="` + base + `"`,
		`hx-swap="delete"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("row is missing %s\n%s", want, body)
		}
	}
	if forms := strings.Count(body, "<form"); forms != 1 {
		t.Errorf("row has %d forms, want 1 -- the row IS the form", forms)
	}
	if strings.Contains(body, `type="submit"`) {
		t.Error("the row renders a submit button")
	}
	if strings.Contains(body, `name="level"`) {
		t.Errorf("the row renders a level control\n%s", body)
	}
}
func TestSpellRowAlwaysRendersEveryControl(t *testing.T) {
	for _, spell := range []Spell{
		{ID: testSpellID, Level: 1, School: DefaultSpellSchool},
		{
			ID: testSpellID, Level: 1, Name: "Shield", School: "Abjuration",
			CastingTime: "Reaction", CastingRange: "Self", Duration: "1 round",
			Components: "V, S", Description: "+5 AC", Prepared: true,
		},
	} {
		var buf bytes.Buffer
		if err := SpellRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", spell).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		body := buf.String()
		for _, name := range []string{
			"name", "school", "components", "casting_time",
			"casting_range", "duration", "description", "prepared",
		} {
			if !strings.Contains(body, `name="`+name+`"`) {
				t.Errorf("prepared=%v: no control named %q\n%s", spell.Prepared, name, body)
			}
		}
		if !strings.Contains(body, `type="checkbox"`) {
			t.Errorf("prepared=%v: the prepared control is not a checkbox", spell.Prepared)
		}
		if checked := strings.Contains(body, "checked"); checked != spell.Prepared {
			t.Errorf("prepared=%v but checked=%v", spell.Prepared, checked)
		}
	}
}
func TestAnUnnamedSpellOpensItsOwnDetails(t *testing.T) {
	for _, c := range []struct {
		name string
		open bool
	}{
		{"", true},
		{"Fireball", false},
	} {
		var buf bytes.Buffer
		spell := Spell{ID: testSpellID, Level: 1, Name: c.name, School: DefaultSpellSchool}
		if err := SpellRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", spell).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		if open := strings.Contains(buf.String(), "<details open"); open != c.open {
			t.Errorf("name=%q: details open=%v, want %v", c.name, open, c.open)
		}
	}
}
func TestNothingLinksToASpellsIndex(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, page := range []struct {
		name   string
		render func() templ.Component
	}{
		{"character", func() templ.Component {
			return EditCharacter(EditCharacterPageData{CharacterID: characterID})
		}},
		{"cantrips", func() templ.Component {
			return EditCharacterSpellLevel(SpellLevelPageData{CharacterID: characterID, Level: 0, Current: testSpellCounters(0)})
		}},
		{"level 3", func() templ.Component {
			return EditCharacterSpellLevel(SpellLevelPageData{CharacterID: characterID, Level: 3, Current: testSpellCounters(3)})
		}},
		{"inventory", func() templ.Component {
			return EditCharacterInventory(InventoryPageData{CharacterID: characterID})
		}},
	} {
		t.Run(page.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := page.render().Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			body := buf.String()
			if strings.Contains(body, `href="/characters/`+characterID+`/edit/spells"`) {
				t.Errorf("%s links to a spells index\n%s", page.name, body)
			}
			if want := `href="/characters/` + characterID + `/edit/spells/0"`; !strings.Contains(body, want) {
				t.Errorf("%s has no way into the spells tab", page.name)
			}
		})
	}
}
func TestSpellLevelTabsAreTenStaticLabels(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	data := SpellLevelPageData{
		CharacterID: characterID,
		Level:       3,
		Current:     testSpellCounters(3),
		Spells: []Spell{
			{ID: testSpellID, Level: 3, Name: "Fireball", School: DefaultSpellSchool},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS3", Level: 3, Name: "Counterspell", School: "Abjuration"},
		},
	}
	if err := EditCharacterSpellLevel(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for level := 0; level <= MaxSpellLevel; level++ {
		want := `href="/characters/` + characterID + `/edit/spells/` + strconv.Itoa(level) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("no way to reach level %d", level)
		}
	}
	if strings.Contains(body, "Overview") {
		t.Errorf("the level strip still carries an Overview tab\n%s", body)
	}
	tabs := body[strings.Index(body, `aria-label="Spell levels"`):]
	tabs = tabs[:strings.Index(tabs, "</nav>")]
	for _, digit := range []string{">2<", ">0<"} {
		if strings.Contains(tabs, digit) {
			t.Errorf("the level tabs carry a count: %s\n%s", digit, tabs)
		}
	}
	for _, want := range []string{">Cantrips<", ">1st<", ">3rd<", ">9th<"} {
		if !strings.Contains(tabs, want) {
			t.Errorf("the level tabs are missing %s\n%s", want, tabs)
		}
	}
}
func TestSpellLevelPagePostsToItsOwnLevel(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	data := SpellLevelPageData{
		CharacterID: characterID,
		Level:       3,
		Current:     testSpellCounters(3),
		Spells: []Spell{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Level: 3, Name: "Fireball", School: DefaultSpellSchool},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Level: 3, Name: "Counterspell", School: "Abjuration"},
		},
	}
	var buf bytes.Buffer
	if err := EditCharacterSpellLevel(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if got := strings.Count(body, "<form"); got != len(data.Spells)+1+closingForms {
		t.Errorf("forms = %d, want %d", got, len(data.Spells)+1+closingForms)
	}
	for _, want := range []string{
		`hx-post="/characters/` + characterID + `/spells/slots/3"`,
		`hx-post="/characters/` + characterID + `/spells/3"`,
		`hx-target="#spell-rows"`,
		`hx-swap="append"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the level page is missing %s\n%s", want, body)
		}
	}
	for _, spell := range data.Spells {
		want := `hx-post="/characters/` + characterID + `/spells/3/` + spell.ID + `"`
		if !strings.Contains(body, want) {
			t.Errorf("%s does not save to %s", spell.Name, want)
		}
	}
	for _, level := range []string{"1", "2", "4", "9"} {
		if strings.Contains(body, "/spells/slots/"+level+`"`) {
			t.Errorf("the level 3 page carries level %s's counters", level)
		}
	}
	assertCharacterTabs(t, body, "/characters/"+characterID+"/edit/spells/0")
	if want := `href="/characters/` + characterID + `/edit/spells/3" aria-current="page"`; !strings.Contains(body, want) {
		t.Errorf("the level tabs do not mark level 3 as current\n%s", body)
	}
}
func TestCantripsPageHasNoSlotCounters(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	data := SpellLevelPageData{
		CharacterID: characterID,
		Level:       0,
		Current:     testSpellCounters(0),
		Spells:      []Spell{{ID: testSpellID, Level: 0, Name: "Fire Bolt", School: DefaultSpellSchool}},
	}
	var buf bytes.Buffer
	if err := EditCharacterSpellLevel(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if got := strings.Count(body, "<form"); got != len(data.Spells)+closingForms {
		t.Errorf("forms = %d, want %d -- cantrips have no slot form", got, len(data.Spells)+closingForms)
	}
	for _, gone := range []string{`name="slots"`, `name="used"`, "/spells/slots/"} {
		if strings.Contains(body, gone) {
			t.Errorf("the cantrips page carries %s\n%s", gone, body)
		}
	}
	if !strings.Contains(body, "Cantrips") {
		t.Errorf("the cantrips page does not name itself\n%s", body)
	}
}
func TestPreparedSpellsIsAViewAndNotAForm(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	groups := []PreparedSpellGroup{
		{Level: 0, Name: "Cantrips", Spells: []Spell{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Level: 0, Name: "Fire Bolt", CastingTime: "Action", CastingRange: "120 feet"},
		}},
		{Level: 3, Name: "Level 3", Spells: []Spell{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Level: 3, Name: "Fireball", CastingTime: "Action", CastingRange: "150 feet", Duration: "Instantaneous"},
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS2", Level: 3, Name: ""},
		}},
	}
	if err := preparedSpells(characterID, groups).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, forbidden := range []string{"<form", "<input", "<textarea", "<select", "hx-post", "hx-delete"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the prepared view carries %s\n%s", forbidden, body)
		}
	}
	for _, want := range []string{"Cantrips", "Level 3", "Fire Bolt", "Fireball"} {
		if !strings.Contains(body, want) {
			t.Errorf("the prepared view is missing %q\n%s", want, body)
		}
	}
	if !strings.Contains(body, "Action \u00b7 150 feet \u00b7 Instantaneous") {
		t.Errorf("the meta line is not rendered\n%s", body)
	}
	if !strings.Contains(body, "Unnamed spell") {
		t.Errorf("an unnamed prepared row renders as nothing\n%s", body)
	}
}
func TestPreparedSpellsEmptyStatePointsAtTheSpellsPage(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	if err := preparedSpells(characterID, nil).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if want := `href="/characters/` + characterID + `/edit/spells/0"`; !strings.Contains(body, want) {
		t.Errorf("the empty state does not point at %s\n%s", want, body)
	}
	if !strings.Contains(body, "Prepared") {
		t.Errorf("the empty state does not name the control that fills it\n%s", body)
	}
}
func TestSpellMetaLineSkipsWhatIsNotThere(t *testing.T) {
	for _, c := range []struct {
		spell Spell
		want  string
	}{
		{Spell{}, ""},
		{Spell{CastingTime: "Action"}, "Action"},
		{Spell{CastingTime: "Action", Duration: "1 minute"}, "Action · 1 minute"},
		{Spell{CastingRange: "Self"}, "Self"},
		{
			Spell{CastingTime: "1 hour", CastingRange: "Touch", Duration: "8 hours"},
			"1 hour · Touch · 8 hours",
		},
	} {
		if got := SpellMetaLine(c.spell); got != c.want {
			t.Errorf("meta line = %q, want %q", got, c.want)
		}
	}
}
func TestCharacterPageRendersBothTickedViews(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	data := EditCharacterPageData{
		CharacterID: id,
		Equipped:    []InventoryItem{{ID: "01BX5ZZKBKACTAV9WEVGEMMVS0", Name: "Chain Mail", Quantity: "1"}},
		Prepared: []PreparedSpellGroup{{Level: 1, Name: "Level 1", Spells: []Spell{
			{ID: "01BX5ZZKBKACTAV9WEVGEMMVS1", Level: 1, Name: "Cure Wounds", CastingTime: "Action"},
		}}},
	}
	if err := EditCharacter(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, want := range []string{"Equipment", "Chain Mail", "Prepared Spells", "Cure Wounds", "Level 1"} {
		if !strings.Contains(body, want) {
			t.Errorf("the Character page is missing %q", want)
		}
	}
	prepared := strings.Index(body, "Prepared Spells")
	equipment := strings.Index(body, "Equipment")
	slots := strings.Index(body, "Spell Slots")
	if !(equipment < prepared && prepared < slots) {
		t.Errorf("panel order is Equipment %d, Prepared %d, Slots %d", equipment, prepared, slots)
	}
}
func testSpellLevels() []SpellLevel {
	levels := make([]SpellLevel, 0, MaxSpellLevel+1)
	for level := 0; level <= MaxSpellLevel; level++ {
		levels = append(levels, SpellLevel{Level: level, Slots: "0", Used: "0"})
	}
	return levels
}
func TestSpellSlotsPanelIsOneFormPerLevelInUse(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	levels := testSpellLevels()
	levels[0].Count = 5
	levels[1].Slots = "4"
	levels[2].Slots = "3"
	levels[3].Slots = "2"
	levels[3].Used = "1"
	levels[3].Count = 2
	inUse := []int{0, 1, 2, 3}
	unused := []int{4, 5, 6, 7, 8, 9}
	var buf bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: id, SpellSlots: levels}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	for _, level := range []int{1, 2, 3} {
		want := `hx-post="/characters/` + id + `/spells/slots/` + strconv.Itoa(level) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("level %d has no slot form posting to %s", level, want)
		}
	}
	if strings.Contains(body, "/spells/slots/0") {
		t.Error("cantrips carry a slot form")
	}
	if !strings.Contains(body, "Unlimited") {
		t.Errorf("cantrips do not say why they have no counters\n%s", body)
	}
	const panels = 10
	if got := strings.Count(body, "<form"); got != panels+3+closingForms {
		t.Errorf("forms = %d, want %d", got, panels+3+closingForms)
	}
	for _, level := range inUse {
		want := `href="/characters/` + id + `/edit/spells/` + strconv.Itoa(level) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("level %d is in use and not linked from the panel", level)
		}
	}
	for _, level := range unused {
		want := `href="/characters/` + id + `/edit/spells/` + strconv.Itoa(level) + `"`
		if strings.Contains(body, want) {
			t.Errorf("level %d has nothing at it and is on the panel", level)
		}
	}
	for _, want := range []string{"5 spells", "2 spells", "No spells"} {
		if !strings.Contains(body, want) {
			t.Errorf("the panel is missing %q", want)
		}
	}
}
func TestActiveSpellLevelsKeepsWhatIsInUse(t *testing.T) {
	for _, c := range []struct {
		name  string
		level SpellLevel
		keep  bool
	}{
		{"nothing at all", SpellLevel{Level: 4, Slots: "0", Used: "0"}, false},
		{"slots set", SpellLevel{Level: 4, Slots: "2", Used: "0"}, true},
		{"every slot spent", SpellLevel{Level: 4, Slots: "2", Used: "2"}, true},
		{"spells but no slots", SpellLevel{Level: 0, Slots: "0", Used: "0", Count: 3}, true},
		{"used without slots", SpellLevel{Level: 4, Slots: "0", Used: "1"}, true},
	} {
		got := activeSpellLevels([]SpellLevel{c.level})
		if kept := len(got) == 1; kept != c.keep {
			t.Errorf("%s: kept = %v, want %v", c.name, kept, c.keep)
		}
	}
	if got := activeSpellLevels(testSpellLevels()); len(got) != 0 {
		t.Errorf("a character with nothing anywhere keeps %d levels, want 0", len(got))
	}
}
func TestSpellSlotsPanelIsAnEmptyStateUntilALevelIsInUse(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const emptyState = "No spell levels in use yet"
	for _, c := range []struct {
		name  string
		build func() []SpellLevel
		shown []int
	}{
		{"nothing at all", testSpellLevels, nil},
		{"one cantrip", func() []SpellLevel {
			l := testSpellLevels()
			l[0].Count = 1
			return l
		}, []int{0}},
		{"a spell at a level with no slots", func() []SpellLevel {
			l := testSpellLevels()
			l[2].Count = 1
			return l
		}, []int{2}},
		{"one slot", func() []SpellLevel {
			l := testSpellLevels()
			l[1].Slots = "2"
			return l
		}, []int{1}},
		{"a slot already spent", func() []SpellLevel {
			l := testSpellLevels()
			l[1].Used = "1"
			return l
		}, []int{1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := spellSlotsOverview(id, c.build()).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			body := buf.String()
			empty := strings.Contains(body, emptyState)
			if empty != (len(c.shown) == 0) {
				t.Errorf("empty state = %v, want %v\n%s", empty, len(c.shown) == 0, body)
			}
			shown := map[int]bool{}
			for _, level := range c.shown {
				shown[level] = true
			}
			for level := 0; level <= MaxSpellLevel; level++ {
				href := `href="/characters/` + id + `/edit/spells/` + strconv.Itoa(level) + `"`
				want := shown[level] || (empty && level == 0)
				if got := strings.Contains(body, href); got != want {
					t.Errorf("level %d linked = %v, want %v\n%s", level, got, want, body)
				}
			}
		})
	}
}
func TestPreparedSpellsClampTheirDescriptions(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const long = "A bright streak flashes from your pointing finger to a point you choose within range and then blossoms with a low roar into an explosion of flame."
	var buf bytes.Buffer
	groups := []PreparedSpellGroup{{Level: 3, Name: "Level 3", Spells: []Spell{
		{ID: testSpellID, Level: 3, Name: "Fireball", CastingTime: "Action", Description: long},
		{ID: "01BX5ZZKBKACTAV9WEVGEMMVS4", Level: 3, Name: "Counterspell", CastingTime: "Reaction"},
	}}}
	if err := preparedSpells(id, groups).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if got := strings.Count(body, "<details"); got != 1 {
		t.Errorf("disclosures = %d, want 1 -- only Fireball has text", got)
	}
	for _, want := range []string{"line-clamp-2", "group-open:line-clamp-none", "cursor-pointer"} {
		if !strings.Contains(body, want) {
			t.Errorf("the description is missing %s\n%s", want, body)
		}
	}
	if !strings.Contains(body, long) {
		t.Errorf("the description was truncated before it reached the markup\n%s", body)
	}
	if !strings.Contains(body, "whitespace-pre-line") {
		t.Errorf("the description collapses its line breaks\n%s", body)
	}
}
func TestAnUnnamedItemOpensItsOwnDetails(t *testing.T) {
	for _, c := range []struct {
		name string
		open bool
	}{
		{"", true},
		{"Longsword", false},
	} {
		var buf bytes.Buffer
		item := InventoryItem{ID: testItemID, Name: c.name, Quantity: "1"}
		if err := InventoryRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", item).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		if open := strings.Contains(buf.String(), "<details open"); open != c.open {
			t.Errorf("name=%q: details open=%v, want %v", c.name, open, c.open)
		}
	}
}
func testJournalEntry() JournalEntry {
	return JournalEntry{
		ID:      testEntryID,
		Title:   "Session 12",
		Created: Timestamp{ISO: "2026-09-05T18:04:11Z", Text: "5 Sep 2026, 18:04 UTC"},
		Updated: Timestamp{ISO: "2026-09-06T09:30:00Z", Text: "6 Sep 2026, 09:30 UTC"},
	}
}

const testEntryID = "01BX5ZZKBKACTAV9WEVGEMMVS1"

func TestJournalEntryPageIsASavingPanel(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	err := EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: characterID,
		EntryID:     testEntryID,
		Title:       "Session 12",
		Body:        "We went back to the marsh.",
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, want := range []string{
		`hx-post="/characters/` + characterID + `/journal/` + testEntryID + `"`,
		`hx-trigger="input delay:1s`,
		`hx-target="#errors-journal"`,
		`hx-status:422="target:#errors-journal,swap:outerHTML"`,
		`<div id="errors-journal" hidden></div>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}
	if !strings.Contains(markup, `We went back to the marsh.</textarea>`) {
		t.Errorf("the body is not in the textarea\n%s", markup)
	}
	if !strings.Contains(markup, `name="body"`) || !strings.Contains(markup, `name="title"`) {
		t.Errorf("the form does not carry both fields\n%s", markup)
	}
	assertCharacterTabs(t, markup, "/characters/"+characterID+"/edit/journal")
}
func TestJournalSaveButtonPostsTheSameForm(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	err := EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: characterID,
		EntryID:     testEntryID,
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	action := "/characters/" + characterID + "/journal/" + testEntryID
	for _, want := range []string{
		`<form id="panel-journal"`,
		`hx-include="#panel-journal"`,
		`hx-vals="{&#34;announce&#34;:&#34;1&#34;}"`,
		`hx-post="` + action + `"`,
		`hx-target="#errors-journal"`,
		`>Save</button>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}
	export := strings.Index(markup, ">Export<")
	save := strings.Index(markup, ">Save</button>")
	tabs := strings.Index(markup, "<nav")
	if export < 0 || save < export || tabs < save {
		t.Errorf("Save is not last in the header action row\n%s", markup)
	}
}
func TestEveryPageHeaderLeadsWithItsBackLink(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for name, c := range map[string]struct {
		page  templ.Component
		href  string
		label string
	}{
		"roster":           {Characters(nil), "/", "Home"},
		"manual":           {Monsters(MonsterListData{}), "/", "Home"},
		"asset manager":    {MapAssets(nil), "/", "Home"},
		"asset tokens":     {TokenAssets(nil), "/", "Home"},
		"asset avatars":    {AvatarAssets(nil), "/", "Home"},
		"asset music":      {MusicAssets(nil), "/", "Home"},
		"character editor": {EditCharacter(EditCharacterPageData{CharacterID: characterID}), "/characters", "Characters"},
		"monster editor":   {EditMonster(EditMonsterPageData{MonsterID: "M", Header: MonsterHeader{MonsterID: "M"}}), "/monsters", "Monsters"},
		"journal tab":      {EditCharacterJournal(JournalPageData{CharacterID: characterID}), "/characters", "Characters"},
		"journal entry": {EditCharacterJournalEntry(JournalEntryPageData{
			CharacterID: characterID,
			EntryID:     testEntryID,
		}), "/characters/" + characterID + "/edit/journal", "Journal"},
	} {
		t.Run(name, func(t *testing.T) {
			body := renderToString(t, c.page)
			at := strings.Index(body, `<a href="`+c.href+`" class="btn shrink-0">`)
			if at < 0 {
				t.Fatalf("no back link to %s:\n%s", c.href, body)
			}
			if !strings.Contains(body[at:], "</svg>"+c.label+"</a>") {
				t.Errorf("the back link does not say %q:\n%s", c.label, body)
			}
			if heading := strings.Index(body, "<h1"); heading >= 0 && at > heading {
				t.Errorf("the back link is not first in the header:\n%s", body)
			}
		})
	}
}
func TestJournalToolbarCannotSubmitTheForm(t *testing.T) {
	var buf bytes.Buffer
	if err := EditCharacterJournalEntry(JournalEntryPageData{}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	buttons := strings.Count(markup, "data-journal-mark=")
	if buttons != 6 {
		t.Errorf("toolbar has %d buttons, want 6", buttons)
	}
	if got := strings.Count(markup, `type="button"`); got < buttons {
		t.Errorf("%d buttons but only %d type=\"button\"\n%s", buttons, got, markup)
	}
	if !strings.Contains(markup, `data-journal-toolbar`) || !strings.Contains(markup, "hidden") {
		t.Errorf("the toolbar is not hidden until the editor mounts\n%s", markup)
	}
	if strings.Contains(markup, `data-journal-heading name=`) || strings.Contains(markup, `name="heading"`) {
		t.Errorf("the heading select is posted with the form\n%s", markup)
	}
}
func TestJournalEditorCarriesItsUploadURL(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	err := EditCharacterJournalEntry(JournalEntryPageData{
		CharacterID: characterID,
		EntryID:     testEntryID,
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	want := `data-journal-images="/characters/` + characterID + `/journal/` + testEntryID + `/images"`
	if !strings.Contains(markup, want) {
		t.Errorf("missing %s\n%s", want, markup)
	}
	if !strings.Contains(markup, "data-journal-upload") {
		t.Errorf("no upload button\n%s", markup)
	}
	if !strings.Contains(markup, `accept="image/png, image/jpeg, image/webp"`) {
		t.Errorf("the file input does not filter the picker\n%s", markup)
	}
	if strings.Contains(markup, `data-journal-upload data-journal-mark`) ||
		strings.Contains(markup, `data-journal-mark data-journal-upload`) {
		t.Errorf("the upload button reports a pressed state it does not have\n%s", markup)
	}
}
func TestJournalLinkFragmentIsADialogWithNoRequest(t *testing.T) {
	var buf bytes.Buffer
	if err := JournalLinkFragment().Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	close := strings.Index(markup, ">Close<")
	insert := strings.Index(markup, ">Insert link<")
	switch {
	case close < 0 || insert < 0:
		t.Fatalf("the dialog is missing one of its buttons\n%s", markup)
	case close > insert:
		t.Errorf("Close comes after the affirmative action\n%s", markup)
	}
	if !strings.Contains(markup, "data-journal-link") {
		t.Errorf("nothing identifies the form to the editor\n%s", markup)
	}
	for _, forbidden := range []string{"hx-post", "hx-get", "hx-trigger"} {
		if strings.Contains(markup, forbidden) {
			t.Errorf("the dialog posts something (%s)\n%s", forbidden, markup)
		}
	}
}
func TestJournalListRendersBothHalvesOfEveryDate(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	err := EditCharacterJournal(JournalPageData{
		CharacterID: characterID,
		Entries:     []JournalEntry{testJournalEntry()},
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, want := range []string{
		`<time datetime="2026-09-05T18:04:11Z">5 Sep 2026, 18:04 UTC</time>`,
		`<time datetime="2026-09-06T09:30:00Z">6 Sep 2026, 09:30 UTC</time>`,
	} {
		if !strings.Contains(collapseWhitespace(markup), want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}
	if strings.Contains(markup, "local-time") {
		t.Errorf("the client-side rewrite is still in the markup\n%s", markup)
	}
	href := `href="/characters/` + characterID + `/edit/journal/` + testEntryID + `"`
	if got := strings.Count(markup, href); got != 2 {
		t.Errorf("the entry is linked %d times, want 2 (the title and View)\n%s", got, markup)
	}
	if !strings.Contains(markup, `>View</a>`) {
		t.Errorf("no View button on the card\n%s", markup)
	}
	if !strings.Contains(markup, `hx-delete="/characters/`+characterID+`/journal/`+testEntryID+`"`) {
		t.Errorf("the delete does not aim at the resource URL\n%s", markup)
	}
	if !strings.Contains(markup, "hx-confirm=") {
		t.Errorf("a delete with no confirmation\n%s", markup)
	}
}
func TestJournalCreateIsAFormPostInTheHeader(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	if err := EditCharacterJournal(JournalPageData{CharacterID: characterID}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	form := `<form method="post" action="/characters/` + characterID + `/journal"`
	at := strings.Index(markup, form)
	if at < 0 {
		t.Fatalf("no create form\n%s", markup)
	}
	if tabs := strings.Index(markup, "<nav"); at > tabs {
		t.Errorf("the create form is below the tabs, not in the header\n%s", markup)
	}
	if !strings.Contains(markup, ">New Entry</button>") {
		t.Errorf("the create button is not labelled\n%s", markup)
	}
	panel := markup[strings.Index(markup, "journal-entries"):strings.Index(markup, "</character-editor>")]
	if strings.Contains(panel, "<form") {
		t.Errorf("a form survives inside the panel\n%s", panel)
	}
}
func TestJournalListNamesTheUnnamedEntry(t *testing.T) {
	var buf bytes.Buffer
	err := EditCharacterJournal(JournalPageData{
		Entries: []JournalEntry{{ID: testEntryID}},
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "Untitled entry") {
		t.Errorf("an unnamed entry renders a blank line\n%s", buf.String())
	}
}
func TestJournalSearchBoxSitsOutsideTheListItSwaps(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	err := EditCharacterJournal(JournalPageData{
		CharacterID: characterID,
		Entries:     []JournalEntry{testJournalEntry()},
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	box := strings.Index(markup, `name="q"`)
	if box < 0 {
		t.Fatalf("no search box\n%s", markup)
	}
	container := `id="` + journalEntriesID + `"`
	list := strings.Index(markup, container)
	if list < 0 {
		t.Fatalf("no list container\n%s", markup)
	}
	if box > list {
		t.Errorf("the search box is inside the list it swaps\n%s", markup)
	}
	if got := strings.Count(markup, container); got != 1 {
		t.Errorf("the list container id appears %d times, want 1", got)
	}
	if !strings.Contains(markup, `hx-target="#`+journalEntriesID+`"`) {
		t.Errorf("the box does not aim at the list\n%s", markup)
	}
	if !strings.Contains(markup, `hx-get="/fragment/character/journal-entries?character=`+characterID+`"`) {
		t.Errorf("the box does not call the fragment route\n%s", markup)
	}
	if !strings.Contains(markup, `maxlength="255"`) {
		t.Errorf("the box is not capped at the length the server accepts\n%s", markup)
	}
}
func TestJournalEmptyListDistinguishesUnwrittenFromUnmatched(t *testing.T) {
	render := func(t *testing.T, data JournalPageData) string {
		t.Helper()
		var buf bytes.Buffer
		if err := EditCharacterJournal(data).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		return buf.String()
	}
	unwritten := render(t, JournalPageData{})
	if !strings.Contains(unwritten, "Nothing written down yet.") {
		t.Errorf("an empty journal does not say so\n%s", unwritten)
	}
	unmatched := render(t, JournalPageData{Query: "hag"})
	if !strings.Contains(unmatched, `No entries match &#34;hag&#34;.`) {
		t.Errorf("a search that missed does not say so\n%s", unmatched)
	}
	if strings.Contains(unmatched, "Nothing written down yet.") {
		t.Errorf("a search that missed reads as an empty journal\n%s", unmatched)
	}
}
func TestJournalSearchFragmentIsNotASecondCopyOfTheList(t *testing.T) {
	data := JournalPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Entries:     []JournalEntry{testJournalEntry()},
		Query:       "hag",
	}
	var page, fragment bytes.Buffer
	if err := EditCharacterJournal(data).Render(context.Background(), &page); err != nil {
		t.Fatalf("render page: %v", err)
	}
	if err := JournalEntriesFragment(data).Render(context.Background(), &fragment); err != nil {
		t.Fatalf("render fragment: %v", err)
	}
	if fragment.Len() == 0 {
		t.Fatal("the fragment rendered nothing")
	}
	if !strings.Contains(page.String(), fragment.String()) {
		t.Errorf("the fragment is not the page's own list\nfragment:\n%s\npage:\n%s", fragment.String(), page.String())
	}
	if strings.Contains(fragment.String(), `id="`+journalEntriesID+`"`) {
		t.Errorf("the fragment carries the container it is swapped into\n%s", fragment.String())
	}
}
func collapseWhitespace(markup string) string {
	return strings.Join(strings.Fields(markup), " ")
}
func TestTheVitalsPanelRendersEveryControlItIsReadFrom(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: id}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, control := range []string{
		"hit_dice", "hit_dice_spent", "death_save_successes",
		"death_save_failures", "heroic_inspiration", "exhaustion",
	} {
		if !strings.Contains(markup, `name="`+control+`"`) {
			t.Errorf("the vitals panel does not render %q, so a save would read it as empty", control)
		}
	}
	for _, row := range []string{"death_save_successes", "death_save_failures"} {
		if got := strings.Count(markup, `name="`+row+`"`); got != DeathSaveLimit {
			t.Errorf("%s renders %d boxes, want %d", row, got, DeathSaveLimit)
		}
	}
	for _, want := range []string{
		`max="` + strconv.Itoa(HitDiceSpentLimit) + `"`,
		`max="` + strconv.Itoa(ExhaustionLimit) + `"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("no vitals counter carries %s", want)
		}
	}
}
func TestDeathSaveBubblesRenderWhatIsStored(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, c := range []struct {
		name string
		data EditCharacterPageData
		want int
	}{
		{"nothing ticked", EditCharacterPageData{CharacterID: id}, 0},
		{"two successes", EditCharacterPageData{CharacterID: id, DeathSaveSuccesses: 2}, 2},
		{"dying, and inspired", EditCharacterPageData{CharacterID: id, DeathSaveSuccesses: 1, DeathSaveFailures: 2, HeroicInspiration: true}, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := EditCharacter(c.data).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Count(buf.String(), " checked"); got != c.want {
				t.Errorf("ticked boxes = %d, want %d", got, c.want)
			}
		})
	}
}

const testAttackRowID = "01BX5ZZKBKACTAV9WEVGEMMVS0"

func TestAttackRowIsItsOwnForm(t *testing.T) {
	const character = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	action := "/characters/" + character + "/attacks/" + testAttackRowID
	var buf bytes.Buffer
	if err := AttackRow(character, Attack{ID: testAttackRowID}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, want := range []string{
		`hx-post="` + action + `"`,
		`hx-delete="` + action + `"`,
		`id="errors-` + AttackRowPanel(testAttackRowID) + `"`,
		`hx-trigger="input delay:1s"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the row is missing %s", want)
		}
	}
	if !strings.Contains(markup, `hx-target="closest form"`) || !strings.Contains(markup, `hx-swap="delete"`) {
		t.Errorf("the delete does not swap out its own row:\n%s", markup)
	}
}
func TestAttackRowAlwaysRendersEveryControl(t *testing.T) {
	var buf bytes.Buffer
	if err := AttackRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", Attack{ID: testAttackRowID}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, control := range []string{"name", "attack_bonus", "damage", "damage_type", "mastery", "notes"} {
		if !strings.Contains(markup, `name="`+control+`"`) {
			t.Errorf("the row does not render %q, so a save would blank the column", control)
		}
	}
}
func TestTwoAttackRowsShareNoElementID(t *testing.T) {
	var buf bytes.Buffer
	for _, id := range []string{testAttackRowID, "01BX5ZZKBKACTAV9WEVGEMMVS3"} {
		if err := AttackRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", Attack{ID: id}).Render(context.Background(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
	}
	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(buf.String(), -1) {
		if seen[match[1]] {
			t.Errorf("two attack rows both render id=%q", match[1])
		}
		seen[match[1]] = true
	}
}
func TestAnUnnamedAttackOpensItsOwnDetails(t *testing.T) {
	for _, c := range []struct {
		name string
		row  Attack
		open bool
	}{
		{"a row that was just added", Attack{ID: testAttackRowID}, true},
		{"a row with a name", Attack{ID: testAttackRowID, Name: "Longsword"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := AttackRow("01ARZ3NDEKTSV4RRFFQ69G5FAV", c.row).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "<details open"); got != c.open {
				t.Errorf("details open = %v, want %v", got, c.open)
			}
		})
	}
}
func TestTheAttackSelectsOfferOnlyWhatTheRulesDefine(t *testing.T) {
	if got := len(damageTypeOptions); got != 14 {
		t.Errorf("damage types = %d, want 14", got)
	}
	if got := len(masteryOptions); got != 9 {
		t.Errorf("mastery properties = %d, want 9", got)
	}
	for _, options := range [][]Option{damageTypeOptions, masteryOptions} {
		if options[0].Value != "" {
			t.Errorf("the first option is %q, want the empty one: most rows have neither", options[0].Value)
		}
		for _, option := range options {
			if normalizeChoice(option.Value, options) != option.Value {
				t.Errorf("the select offers %q and the normaliser refuses it", option.Value)
			}
		}
	}
	if NormalizeMastery("Slashing") != "" {
		t.Error("a damage type passes as a mastery property")
	}
	if NormalizeDamageType("Topple") != "" {
		t.Error("a mastery property passes as a damage type")
	}
}
func testDerivedValues() Derived {
	d := Derived{
		StrMod: "+2", DexMod: "+3", ConMod: "+2", IntMod: "-1", WisMod: "+1", ChaMod: "-1",
		PassivePerception: "14", SpellSaveDC: "15", SpellAttackBonus: "+7",
	}
	for _, entry := range SkillEntries() {
		d.Skills = append(d.Skills, BonusRow{Key: entry.Key, Label: entry.Label, Abbr: entry.Abbr, Proficiency: ProficiencyNone, Misc: "0", Total: "+1"})
	}
	for _, entry := range SavingThrowEntries() {
		d.SavingThrows = append(d.SavingThrows, BonusRow{Key: entry.Key, Label: entry.Label, Proficiency: ProficiencyNone, Misc: "0", Total: "+1"})
	}
	return d
}
func TestEveryDerivedValueHasATargetOnThePage(t *testing.T) {
	derived := testDerivedValues()
	var page bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Derived: derived}).Render(context.Background(), &page); err != nil {
		t.Fatalf("render: %v", err)
	}
	var block bytes.Buffer
	if err := DerivedValues(derived).Render(context.Background(), &block); err != nil {
		t.Fatalf("render: %v", err)
	}
	ids := regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(block.String(), -1)
	if want := 6 + len(SkillEntries()) + len(SavingThrowEntries()) + 3; len(ids) != want {
		t.Errorf("the refresh carries %d values, want %d", len(ids), want)
	}
	for _, match := range ids {
		if !strings.Contains(page.String(), `id="`+match[1]+`"`) {
			t.Errorf("the refresh swaps #%s, which the page does not render", match[1])
		}
	}
	if got := strings.Count(block.String(), `hx-swap-oob="true"`); got != len(ids) {
		t.Errorf("%d of %d refreshed values are out-of-band", got, len(ids))
	}
	if strings.Contains(page.String(), "hx-swap-oob") {
		t.Error("the page renders a derived value already marked out-of-band")
	}
}
func TestTheRefreshDrawsEveryDerivedValueTheWayThePageDid(t *testing.T) {
	derived := testDerivedValues()
	var page bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Derived: derived}).Render(context.Background(), &page); err != nil {
		t.Fatalf("render: %v", err)
	}
	var block bytes.Buffer
	if err := DerivedValues(derived).Render(context.Background(), &block); err != nil {
		t.Fatalf("render: %v", err)
	}
	shapes := regexp.MustCompile(`id="([^"]+)" class="([^"]+)"`)
	onThePage := map[string]string{}
	for _, match := range shapes.FindAllStringSubmatch(page.String(), -1) {
		onThePage[match[1]] = match[2]
	}
	refreshed := shapes.FindAllStringSubmatch(block.String(), -1)
	if len(refreshed) == 0 {
		t.Fatal("read no shapes out of the refresh")
	}
	for _, match := range refreshed {
		id, class := match[1], match[2]
		was, ok := onThePage[id]
		if !ok {
			t.Errorf("the refresh swaps #%s, which the page does not render", id)
			continue
		}
		if was != class {
			t.Errorf("#%s is drawn as %q on the page and %q by the refresh", id, was, class)
		}
	}
}
func TestBothNavsRenderInsideTheBar(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	var buf bytes.Buffer
	page := EditCharacterSpellLevel(SpellLevelPageData{
		CharacterID: id,
		Header:      testCharacterHeader(),
		Level:       3,
		Current:     SpellLevel{Level: 3, Slots: "2", Used: "1"},
	})
	if err := page.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	rendered := buf.String()
	barCloses := strings.Index(rendered, "</header>")
	if barCloses < 0 {
		t.Fatal("the page renders no bar at all")
	}
	for _, nav := range []string{`aria-label="Character sheet sections"`, `aria-label="Spell levels"`} {
		at := strings.Index(rendered, nav)
		if at < 0 {
			t.Errorf("%s is not on the page", nav)
			continue
		}
		if at > barCloses {
			t.Errorf("%s renders below the bar, which puts it on the grid paper", nav)
		}
	}
}

var panelBorder = regexp.MustCompile(`border(-[a-z])?-2 border-base-300`)

func TestNothingWearsThePanelBorderAnyMore(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for name, page := range map[string]templ.Component{
		"character": EditCharacter(EditCharacterPageData{
			CharacterID: id,
			Header:      testCharacterHeader(),
			Derived:     testDerivedValues(),
			Features:    []Feature{{Name: "Pact of the Blade"}},
			Attacks:     []Attack{{ID: id, Name: "Pact Blade"}},
			Equipped:    []InventoryItem{{ID: id, Name: "Studded Leather", Quantity: "1"}},
			Prepared:    []PreparedSpellGroup{{Level: 1, Name: "1st Level", Spells: []Spell{{ID: id, Name: "Hex"}}}},
			SpellSlots:  []SpellLevel{{Level: 1, Slots: "2", Used: "1", Count: 1}},
		}),
		"inventory": EditCharacterInventory(InventoryPageData{
			CharacterID: id, Header: testCharacterHeader(),
			Items: []InventoryItem{{ID: id, Name: "Rope", Quantity: "1"}},
		}),
		"spells": EditCharacterSpellLevel(SpellLevelPageData{
			CharacterID: id, Header: testCharacterHeader(), Level: 1,
			Current: SpellLevel{Level: 1, Slots: "2", Used: "1"},
			Spells:  []Spell{{ID: id, Level: 1, Name: "Hex"}},
		}),
		"journal": EditCharacterJournal(JournalPageData{
			CharacterID: id, Header: testCharacterHeader(),
			Entries: []JournalEntry{testJournalEntry()},
		}),
		"roster": Characters([]queries.Character{{ID: ulid.MustParse(id), Name: "Vashti"}}),
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := page.Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if match := panelBorder.FindString(buf.String()); match != "" {
				t.Errorf("something on this page still wears %q, the old panel border, which flattens whatever it sits in", match)
			}
		})
	}
}
func testCharacterHeader() CharacterHeader {
	return CharacterHeader{
		Name:        "Vashti Emberlane",
		Subtitle:    "Tiefling \u00b7 Warlock 5 \u00b7 Soldier \u00b7 Chaotic Good",
		AC:          "15",
		CurrentHP:   "31",
		MaxHP:       "38",
		Speed:       "30 ft.",
		Initiative:  "+3",
		Proficiency: "+3",
		Passive:     "13",
	}
}
func TestTheBarRefreshHasATargetOnThePage(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	header := testCharacterHeader()
	var page bytes.Buffer
	if err := EditCharacter(EditCharacterPageData{CharacterID: id, Header: header}).Render(context.Background(), &page); err != nil {
		t.Fatalf("render: %v", err)
	}
	var block bytes.Buffer
	if err := CharacterBarValues(header).Render(context.Background(), &block); err != nil {
		t.Fatalf("render: %v", err)
	}
	ids := regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(block.String(), -1)
	if len(ids) != 2 {
		t.Fatalf("the refresh carries %d blocks, want 2", len(ids))
	}
	for _, match := range ids {
		if !strings.Contains(page.String(), `id="`+match[1]+`"`) {
			t.Errorf("the refresh swaps #%s, which the page does not render", match[1])
		}
	}
	if got := strings.Count(block.String(), `hx-swap-oob="true"`); got != len(ids) {
		t.Errorf("%d of %d refreshed blocks are out-of-band", got, len(ids))
	}
}
func TestEveryEditorTabSaysWhoseSheetItIs(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	header := testCharacterHeader()
	for name, page := range map[string]templ.Component{
		"character": EditCharacter(EditCharacterPageData{CharacterID: id, Header: header}),
		"inventory": EditCharacterInventory(InventoryPageData{CharacterID: id, Header: header}),
		"spells":    EditCharacterSpellLevel(SpellLevelPageData{CharacterID: id, Header: header}),
		"journal":   EditCharacterJournal(JournalPageData{CharacterID: id, Header: header}),
		"entry":     EditCharacterJournalEntry(JournalEntryPageData{CharacterID: id, Header: header}),
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := page.Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			rendered := buf.String()
			if !strings.Contains(rendered, header.Name) {
				t.Error("the bar does not name the character")
			}
			if !strings.Contains(rendered, header.Subtitle) {
				t.Error("the bar does not carry the subtitle")
			}
			if strings.Contains(rendered, ">Edit Character<") {
				t.Error("the bar still heads the page with the name of the screen")
			}
		})
	}
}
func TestBonusRowsShareNoElementIDButTheirTotals(t *testing.T) {
	var buf bytes.Buffer
	if err := skillsTable(testDerivedValues().Skills, "14").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(buf.String(), -1) {
		if seen[match[1]] {
			t.Errorf("two rows both render id=%q", match[1])
		}
		seen[match[1]] = true
		if !strings.HasPrefix(match[1], "total-") && match[1] != "passive-perception" {
			t.Errorf("a bonus row renders id=%q, which is a control rather than a derived value", match[1])
		}
	}
}
func TestEveryBonusRowPostsBothOfItsHalves(t *testing.T) {
	var buf bytes.Buffer
	if err := skillsTable(testDerivedValues().Skills, "14").Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()
	for _, entry := range SkillEntries() {
		for _, half := range []string{"-misc", "-proficiency"} {
			if want := `name="skills-` + entry.Key + half + `"`; !strings.Contains(markup, want) {
				t.Errorf("the %s row does not carry %s", entry.Key, want)
			}
		}
	}
}
func TestBothThemesPinTheSameBorderWidth(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "css", "app.css"))
	if err != nil {
		t.Fatalf("read app.css: %v", err)
	}
	matches := regexp.MustCompile(`(?m)^\s*--border:\s*([^;]+);`).FindAllStringSubmatch(string(source), -1)
	if len(matches) != 2 {
		t.Fatalf("app.css declares --border %d times, want one per theme", len(matches))
	}
	if matches[0][1] != matches[1][1] {
		t.Errorf("the themes pin --border to %q and %q, so every control changes thickness with the OS theme", matches[0][1], matches[1][1])
	}
}

const (
	testMapID  = "01BX5ZZKBKACTAV9WEVGEMMVS2"
	testMapGen = "01BX5ZZKBKACTAV9WEVGEMMVS3"
)

func testMapCard() MapAsset {
	return MapAsset{
		ID:         testMapID,
		Name:       "The Sunless Citadel",
		FileName:   "citadel.png",
		Generation: testMapGen,
		State:      queries.AssetsTileStateReady,
	}
}
func markup(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func TestTheThreeQuestionsAMapCardAsksAreIndependent(t *testing.T) {
	for name, c := range map[string]struct {
		generation                 string
		state                      queries.AssetsTileState
		usable, polling, retryable bool
	}{
		"just uploaded":                   {"", queries.AssetsTileStatePending, false, true, false},
		"being tiled for the first time":  {"", queries.AssetsTileStateWorking, false, true, false},
		"tiled and idle":                  {testMapGen, queries.AssetsTileStateReady, true, false, false},
		"replacement queued over a map":   {testMapGen, queries.AssetsTileStatePending, true, true, false},
		"replacement being built":         {testMapGen, queries.AssetsTileStateWorking, true, true, false},
		"first build gave up":             {"", queries.AssetsTileStateFailed, false, false, true},
		"rebuild gave up over a good map": {testMapGen, queries.AssetsTileStateFailed, true, false, true},
		"a row from before there was one": {"", "", false, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			m := MapAsset{ID: testMapID, Generation: c.generation, State: c.state}
			if m.Usable() != c.usable {
				t.Errorf("Usable() = %v, want %v", m.Usable(), c.usable)
			}
			if m.Polling() != c.polling {
				t.Errorf("Polling() = %v, want %v", m.Polling(), c.polling)
			}
			if m.Retryable() != c.retryable {
				t.Errorf("Retryable() = %v, want %v", m.Retryable(), c.retryable)
			}
		})
	}
}
func TestOnlyACardWithAJobRunningPollsItself(t *testing.T) {
	poll := []string{
		`hx-get="/fragment/assets/maps/` + testMapID + `/card"`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerMorph"`,
		"data-quiet",
	}
	for name, c := range map[string]struct {
		state queries.AssetsTileState
		want  bool
	}{
		"queued":    {queries.AssetsTileStatePending, true},
		"running":   {queries.AssetsTileStateWorking, true},
		"finished":  {queries.AssetsTileStateReady, false},
		"gave up":   {queries.AssetsTileStateFailed, false},
		"never ran": {"", false},
	} {
		t.Run(name, func(t *testing.T) {
			card := testMapCard()
			card.State = c.state
			rendered := markup(t, MapCard(card))
			for _, attribute := range poll {
				if strings.Contains(rendered, attribute) != c.want {
					t.Errorf("contains %q = %v, want %v\n%s", attribute, !c.want, c.want, rendered)
				}
			}
		})
	}
}
func TestACardBeingRebuiltStillShowsTheMapItHas(t *testing.T) {
	card := testMapCard()
	card.State = queries.AssetsTileStatePending
	rendered := markup(t, MapCard(card))
	if !strings.Contains(rendered, `src="/assets/images/`+testMapID+`/preview"`) {
		t.Errorf("the card dropped its preview while a replacement builds\n%s", rendered)
	}
	if !strings.Contains(rendered, `hx-trigger="every 2s"`) {
		t.Errorf("the card is not polling for the replacement\n%s", rendered)
	}
}
func TestACardWithNoPyramidAsksForNoPreview(t *testing.T) {
	card := testMapCard()
	card.Generation = ""
	card.State = queries.AssetsTileStatePending
	if rendered := markup(t, MapCard(card)); strings.Contains(rendered, "/preview") {
		t.Errorf("the card asks for a preview that does not exist yet\n%s", rendered)
	}
}
func TestOnlyACardThatGaveUpOffersARetry(t *testing.T) {
	retry := `hx-post="/assets/maps/` + testMapID + `/tiles"`
	for name, c := range map[string]struct {
		state queries.AssetsTileState
		want  bool
	}{
		"gave up":  {queries.AssetsTileStateFailed, true},
		"queued":   {queries.AssetsTileStatePending, false},
		"running":  {queries.AssetsTileStateWorking, false},
		"finished": {queries.AssetsTileStateReady, false},
	} {
		t.Run(name, func(t *testing.T) {
			card := testMapCard()
			card.State = c.state
			rendered := markup(t, MapCard(card))
			if strings.Contains(rendered, retry) != c.want {
				t.Errorf("offers the retry = %v, want %v\n%s", !c.want, c.want, rendered)
			}
		})
	}
}
func TestTheNameInputIsIdentifiedAcrossASwap(t *testing.T) {
	rendered := markup(t, MapCard(testMapCard()))
	if !strings.Contains(rendered, `id="map-name-`+testMapID+`"`) {
		t.Errorf("the name input has no id, so a poll would take the caret with it\n%s", rendered)
	}
}
func TestThePageIsMadeOfTheSameCardTheFragmentServes(t *testing.T) {
	card := testMapCard()
	page := markup(t, MapAssets([]MapAsset{card}))
	fragment := markup(t, MapCard(card))
	if fragment == "" {
		t.Fatal("the card rendered nothing")
	}
	if !strings.Contains(page, fragment) {
		t.Errorf("the page's card is not the one the fragment serves\ncard:\n%s\npage:\n%s", fragment, page)
	}
}
func TestEveryAssetPageOffersEveryKind(t *testing.T) {
	kinds := []string{"/assets/maps", "/assets/tokens", "/assets/avatars", "/assets/music"}
	for name, c := range map[string]struct {
		page    templ.Component
		current string
	}{
		"maps":    {MapAssets(nil), "/assets/maps"},
		"tokens":  {TokenAssets(nil), "/assets/tokens"},
		"avatars": {AvatarAssets(nil), "/assets/avatars"},
		"music":   {MusicAssets(nil), "/assets/music"},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.page)
			for _, href := range kinds {
				if !strings.Contains(body, `href="`+href+`"`) {
					t.Errorf("no way to reach %s from here", href)
				}
			}
			if want := `href="` + c.current + `" aria-current="page"`; !strings.Contains(body, want) {
				t.Errorf("the current tab is not %s", c.current)
			}
			if n := strings.Count(body, `aria-current="page"`); n != 1 {
				t.Errorf("%d tabs are marked current, want 1", n)
			}
		})
	}
}
func TestTheAssetTabsRenderInsideTheBar(t *testing.T) {
	body := markup(t, TokenAssets(nil))
	barCloses := strings.Index(body, "</header>")
	if barCloses < 0 {
		t.Fatal("the page renders no bar at all")
	}
	at := strings.Index(body, `aria-label="Asset kinds"`)
	if at < 0 {
		t.Fatal("the page renders no asset nav at all")
	}
	if at > barCloses {
		t.Error("the asset nav renders below the bar, which puts it on the grid paper")
	}
}
func TestTheEmptyStateSitsLastInAGridThatIsAlwaysThere(t *testing.T) {
	const marker = `class="col-span-full hidden only:block"`
	for name, c := range map[string]struct {
		page  templ.Component
		cards bool
	}{
		"a library with nothing in it": {MapAssets(nil), false},
		"a library with a map in it":   {MapAssets([]MapAsset{testMapCard()}), true},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.page)
			if !strings.Contains(body, `id="maps"`) {
				t.Fatal("no #maps section, so the upload has nothing to swap into")
			}
			empty := strings.Index(body, marker)
			if empty < 0 {
				t.Fatalf("the empty state is not rendered:\n%s", body)
			}
			if !strings.Contains(body, "No maps yet.") {
				t.Error("the empty state says nothing")
			}
			if c.cards {
				card := strings.Index(body, "<asset-card")
				if card < 0 {
					t.Fatal("the map did not render a card")
				}
				if card > empty {
					t.Error("the empty state renders before the cards, so it is never an only child")
				}
			}
		})
	}
}
func TestEachKindsEmptyStateNamesItsOwnKind(t *testing.T) {
	for name, c := range map[string]struct {
		page    templ.Component
		heading string
	}{
		"maps":    {MapAssets(nil), "No maps yet."},
		"tokens":  {TokenAssets(nil), "No tokens yet."},
		"avatars": {AvatarAssets(nil), "No avatars yet."},
		"music":   {MusicAssets(nil), "No music yet."},
	} {
		t.Run(name, func(t *testing.T) {
			if body := markup(t, c.page); !strings.Contains(body, c.heading) {
				t.Errorf("the %s page does not say %q", name, c.heading)
			}
		})
	}
}
func TestEveryAssetPageSearchesItsOwnKind(t *testing.T) {
	for kind, page := range map[string]templ.Component{
		"maps":    MapAssets(nil),
		"tokens":  TokenAssets(nil),
		"avatars": AvatarAssets(nil),
		"music":   MusicAssets(nil),
	} {
		t.Run(kind, func(t *testing.T) {
			body := markup(t, page)
			for _, want := range []string{
				`hx-get="/fragment/assets/list?kind=` + kind + `"`,
				`hx-target="#` + kind + `"`,
				`id="` + kind + `"`,
				`name="q"`,
				`maxlength="` + strconv.Itoa(AssetNameLimit) + `"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the %s search box carries no %s", kind, want)
				}
			}
			if n := strings.Count(body, `hx-get="/fragment/assets/list`); n != 1 {
				t.Errorf("%d search boxes on the %s page, want 1", n, kind)
			}
		})
	}
}
func TestASearchThatMatchedNothingRepeatsTheTermBack(t *testing.T) {
	for name, c := range map[string]struct {
		cards   templ.Component
		heading string
		match   string
	}{
		"maps":    {MapCards(nil, "keep"), "No maps yet.", `No maps match "keep".`},
		"tokens":  {TokenCards(nil, "wagon"), "No tokens yet.", `No tokens match "wagon".`},
		"avatars": {AvatarCards(nil, "elf"), "No avatars yet.", `No avatars match "elf".`},
		"music":   {MusicCards(nil, "rain"), "No music yet.", `No tracks match "rain".`},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.cards)
			if !strings.Contains(body, `class="col-span-full hidden only:block"`) {
				t.Fatalf("the search result does not use the empty slot:\n%s", body)
			}
			if want := strings.ReplaceAll(c.match, `"`, "&#34;"); !strings.Contains(body, want) {
				t.Errorf("the term is not repeated back as %q:\n%s", c.match, body)
			}
			if strings.Contains(body, c.heading) {
				t.Errorf("a search that found nothing says the shelf is empty: %q", c.heading)
			}
		})
	}
}
func TestEveryEmptyListSpeaksFromTheSamePanel(t *testing.T) {
	for name, c := range map[string]struct {
		cards templ.Component
		match string
	}{
		"maps searched":     {MapCards(nil, "keep"), `No maps match "keep".`},
		"tokens searched":   {TokenCards(nil, "wagon"), `No tokens match "wagon".`},
		"avatars searched":  {AvatarCards(nil, "elf"), `No avatars match "elf".`},
		"music searched":    {MusicCards(nil, "rain"), `No tracks match "rain".`},
		"monsters searched": {MonsterCardsFragment(MonsterListData{Query: "goblin"}), `No monsters match "goblin".`},
		"maps empty":        {MapCards(nil, ""), ""},
		"tokens empty":      {TokenCards(nil, ""), ""},
		"avatars empty":     {AvatarCards(nil, ""), ""},
		"music empty":       {MusicCards(nil, ""), ""},
		"monsters empty":    {MonsterCardsFragment(MonsterListData{}), ""},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.cards)
			for _, want := range []string{
				sheetSurface,
				"mx-auto",
				"max-w-md",
				"text-center",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the message is missing %q:\n%s", want, body)
				}
			}
			if c.match == "" {
				if strings.Contains(body, noMatchHint) {
					t.Error("a list that was never searched offers to clear the search box")
				}
				return
			}
			if want := strings.ReplaceAll(c.match, `"`, "&#34;"); !strings.Contains(body, want) {
				t.Errorf("the term is not repeated back as %q:\n%s", c.match, body)
			}
			if !strings.Contains(body, noMatchHint) {
				t.Error("the message does not say what to do about it")
			}
		})
	}
}
func TestTheAssetSearchFragmentIsTheSectionThePageAlreadyHas(t *testing.T) {
	for name, c := range map[string]struct {
		page  templ.Component
		cards templ.Component
	}{
		"maps":    {MapAssets(nil), MapCards(nil, "")},
		"tokens":  {TokenAssets(nil), TokenCards(nil, "")},
		"avatars": {AvatarAssets(nil), AvatarCards(nil, "")},
		"music":   {MusicAssets(nil), MusicCards(nil, "")},
	} {
		t.Run(name, func(t *testing.T) {
			section := markup(t, c.cards)
			if !strings.HasPrefix(strings.TrimSpace(section), "<section") {
				t.Fatalf("the fragment is not a bare section:\n%s", section)
			}
			if !strings.Contains(markup(t, c.page), section) {
				t.Errorf("the fragment is not the section the page renders:\n%s", section)
			}
		})
	}
}
func TestTheAssetGridsSizeTheCardRatherThanCountColumns(t *testing.T) {
	for name, c := range map[string]struct {
		page templ.Component
		want string
	}{
		"maps":    {MapAssets(nil), assetTileGrid},
		"tokens":  {TokenAssets(nil), assetTileGrid},
		"avatars": {AvatarAssets(nil), assetFaceGrid},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.page)
			if !strings.Contains(body, c.want) {
				t.Errorf("the %s grid is not %s", name, c.want)
			}
			if strings.Contains(body, "auto-fill") != strings.Contains(c.want, "auto-fill") {
				t.Errorf("the %s grid counts columns rather than sizing cards", name)
			}
		})
	}
	if assetFaceGrid == assetTileGrid {
		t.Error("the avatar wall is the same track as the map tiles")
	}
}
func TestTheAssetNameBoxIsBoundedByTheColumn(t *testing.T) {
	body := markup(t, MapCard(testMapCard()))
	if want := `maxlength="` + strconv.Itoa(AssetNameLimit) + `"`; !strings.Contains(body, want) {
		t.Errorf("the name box carries no %s:\n%s", want, body)
	}
}
func testLibraryCard(kind string) LibraryAsset {
	return LibraryAsset{
		ID:       testMapID,
		Name:     "Rowboat",
		FileName: "rowboat.png",
		Kind:     kind,
		Width:    512,
		Height:   171,
	}
}
func TestALibraryCardOnlyEverAddressesItsOwnKind(t *testing.T) {
	for _, kind := range []string{"tokens", "avatars"} {
		t.Run(kind, func(t *testing.T) {
			body := markup(t, LibraryAssetCard(testLibraryCard(kind)))
			base := "/assets/" + kind + "/" + testMapID
			for _, want := range []string{
				`hx-post="` + base + `"`,
				`hx-delete="` + base + `"`,
				`hx-patch="` + base + `/name"`,
				`src="/assets/images/` + testMapID + `"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the card carries no %s:\n%s", want, body)
				}
			}
			for _, other := range []string{"/assets/maps/", "/assets/music/"} {
				if strings.Contains(body, other) {
					t.Errorf("the card reaches into %s", other)
				}
			}
		})
	}
}
func TestALibraryCardShowsTheWholePicture(t *testing.T) {
	body := markup(t, LibraryAssetCard(testLibraryCard("tokens")))
	if !strings.Contains(body, "object-contain") {
		t.Errorf("the card crops its picture:\n%s", body)
	}
}
func TestTheMapAndLibraryCardsShareTheirControls(t *testing.T) {
	mapCard := markup(t, MapCard(testMapCard()))
	libraryCard := markup(t, LibraryAssetCard(testLibraryCard("tokens")))
	for _, shared := range []string{
		`hx-trigger="input changed delay:1s"`,
		`hx-swap="none"`,
		`hx-target="closest asset-card"`,
		`hx-swap="delete"`,
		"You are about to delete ",
		`maxlength="` + strconv.Itoa(AssetNameLimit) + `"`,
	} {
		if !strings.Contains(mapCard, shared) {
			t.Errorf("the map card is missing %s", shared)
		}
		if !strings.Contains(libraryCard, shared) {
			t.Errorf("the library card is missing %s", shared)
		}
	}
}
func TestALibraryUploadTargetsItsOwnGrid(t *testing.T) {
	for name, c := range map[string]struct {
		page templ.Component
		kind string
	}{
		"tokens":  {TokenAssets(nil), "tokens"},
		"avatars": {AvatarAssets(nil), "avatars"},
	} {
		t.Run(name, func(t *testing.T) {
			body := markup(t, c.page)
			for _, want := range []string{
				`hx-post="/assets/` + c.kind + `"`,
				`hx-target="#` + c.kind + `"`,
				`hx-swap="afterbegin"`,
				`id="` + c.kind + `"`,
				`for="` + c.kind + `-upload"`,
				`id="` + c.kind + `-upload"`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("the %s page carries no %s:\n%s", c.kind, want, body)
				}
			}
		})
	}
}
func testMusicTrack() MusicTrack {
	return MusicTrack{
		ID:       testMapID,
		Name:     "Tavern, evening",
		FileName: "tavern-evening.mp3",
	}
}
func TestAMusicCardPlaysThroughThisServer(t *testing.T) {
	body := markup(t, MusicCard(testMusicTrack()))
	if want := `src="/assets/music/` + testMapID + `/audio"`; !strings.Contains(body, want) {
		t.Errorf("the player does not point at %s:\n%s", want, body)
	}
	if strings.Contains(body, "X-Amz-Signature") || strings.Contains(body, "r2.cloudflarestorage.com") {
		t.Errorf("a signed URL was rendered into the page, and it will expire:\n%s", body)
	}
}
func TestAMusicPageDoesNotStartDownloadingEveryTrack(t *testing.T) {
	body := markup(t, MusicAssets([]MusicTrack{testMusicTrack()}))
	if !strings.Contains(body, `preload="none"`) {
		t.Errorf("the players preload, so opening the page pulls every track:\n%s", body)
	}
}
func TestTheMusicUploadRendersItsOwnControls(t *testing.T) {
	body := markup(t, MusicAssets(nil))
	for _, want := range []string{
		`src="/js/music-upload.js"`,
		"data-music-input",
		"data-music-label",
		"data-music-progress",
		"data-music-bar",
		"data-music-percent",
		`id="music"`,
		"hidden",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the music page carries no %s:\n%s", want, body)
		}
	}
	upload := strings.Index(body, "data-music-input")
	if upload < 0 {
		t.Fatal("no upload input at all")
	}
	if strings.Contains(body[:upload], `hx-post="/assets/music"`) {
		t.Error("the file input posts to the server, so the bytes would not go to the bucket")
	}
}
func TestAMusicCardOffersNoReplace(t *testing.T) {
	body := markup(t, MusicCard(testMusicTrack()))
	if strings.Contains(body, `type="file"`) {
		t.Errorf("the card offers a replace it has no route for:\n%s", body)
	}
	if want := `hx-delete="/assets/music/` + testMapID + `"`; !strings.Contains(body, want) {
		t.Errorf("the card carries no %s:\n%s", want, body)
	}
	if want := `hx-patch="/assets/music/` + testMapID + `/name"`; !strings.Contains(body, want) {
		t.Errorf("the card carries no %s:\n%s", want, body)
	}
}
func TestTheMapPickerDoesNotLinkOutOfTheRoom(t *testing.T) {
	var buf bytes.Buffer
	data := RoomMapsData{
		RoomID:  "room",
		LayerID: "layer",
		Maps: []RoomMapChoice{{
			RoomID: "room", LayerID: "layer", ID: "map",
			Name: "Death House", FileName: "Death House",
			Generation: "gen", Width: 4000, Height: 3000,
		}},
	}
	if err := RoomMaps(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if strings.Contains(body, `href="/assets`) {
		t.Errorf("the picker links to the asset manager:\n%s", body)
	}
	if !strings.Contains(body, `hx-post="/rooms/room/layers/layer/maps"`) || !strings.Contains(body, `type="file"`) {
		t.Errorf("the picker cannot upload a map:\n%s", body)
	}
	if !strings.Contains(body, `hx-target="#`+RoomMapListID+`"`) || !strings.Contains(body, `hx-swap="afterbegin"`) {
		t.Errorf("the upload does not land at the front of the grid:\n%s", body)
	}
	if !strings.Contains(body, `id="`+RoomMapListID+`"`) || !strings.Contains(body, `type="search"`) {
		t.Errorf("the picker has no search:\n%s", body)
	}
}
func TestAPickerCardPollsUntilItsTilesAreReady(t *testing.T) {
	building := RoomMapChoice{
		RoomID: "room", LayerID: "layer", ID: "map",
		Name: "keep.png", FileName: "keep.png", State: queries.AssetsTileStatePending,
	}
	var buf bytes.Buffer
	if err := RoomMapCard(building).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()
	if !strings.Contains(body, `hx-get="`+escapeAmps(building.CardURL())+`"`) || !strings.Contains(body, `hx-trigger="every 2s"`) {
		t.Errorf("a building card does not ask again:\n%s", body)
	}
	if !strings.Contains(body, "loading-spinner") || !strings.Contains(body, "Building tiles") {
		t.Errorf("a building card does not say so:\n%s", body)
	}
	if strings.Contains(body, `name="asset"`) {
		t.Errorf("a map with no tiles is offered as a choice:\n%s", body)
	}
	ready := building
	ready.Generation = "gen"
	ready.State = ""
	ready.Width, ready.Height = 4000, 3000
	buf.Reset()
	if err := RoomMapCard(ready).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body = buf.String()
	if strings.Contains(body, "hx-trigger") {
		t.Errorf("a finished card goes on polling:\n%s", body)
	}
	if !strings.Contains(body, ready.PreviewURL()) {
		t.Errorf("a finished card has no preview:\n%s", body)
	}
	if !strings.Contains(body, `name="asset" value="map"`) || !strings.Contains(body, `hx-post="`+ready.SetPath()+`"`) {
		t.Errorf("a finished card is not a button:\n%s", body)
	}
}
func escapeAmps(s string) string { return strings.ReplaceAll(s, "&", "&amp;") }
func TestAFailureThatWillBeRetriedDoesNotSayItGaveUp(t *testing.T) {
	manager := MapAsset{ID: "m", Name: "castle.png", State: queries.AssetsTileStateFailed}
	picker := RoomMapChoice{RoomID: "r", LayerID: "l", ID: "m", Name: "castle.png", State: queries.AssetsTileStateFailed}
	for name, tc := range map[string]struct {
		autoRetry bool
		want      string
		notWant   string
		label     string
		notLabel  string
	}{
		"the worker is coming back": {autoRetry: true, want: tilingRetrying, notWant: tilingGaveUp, label: "Try now", notLabel: "Try again"},
		"the worker has stopped":    {autoRetry: false, want: tilingGaveUp, notWant: tilingRetrying, label: "Try again", notLabel: "Try now"},
	} {
		t.Run(name, func(t *testing.T) {
			manager.AutoRetry, picker.AutoRetry = tc.autoRetry, tc.autoRetry
			for card, body := range map[string]string{
				"the asset manager": renderString(t, MapCard(manager)),
				"the map picker":    renderString(t, RoomMapCard(picker)),
			} {
				if !strings.Contains(body, tc.want) || strings.Contains(body, tc.notWant) {
					t.Errorf("%s does not say %q and only that:\n%s", card, tc.want, body)
				}
				if !strings.Contains(body, tc.label) || strings.Contains(body, tc.notLabel) {
					t.Errorf("%s does not offer %q and only that:\n%s", card, tc.label, body)
				}
			}
		})
	}
	if tilingGaveUp != "Tiling gave up." {
		t.Errorf("the final wording moved: %q", tilingGaveUp)
	}
}
func TestOnlyAFailedCardTalksAboutRetrying(t *testing.T) {
	for name, m := range map[string]MapAsset{
		"building": {ID: "m", Name: "castle.png", State: queries.AssetsTileStatePending},
		"finished": {ID: "m", Name: "castle.png", Generation: "gen"},
	} {
		t.Run(name, func(t *testing.T) {
			body := renderString(t, MapCard(m))
			if strings.Contains(body, "Tiling") {
				t.Errorf("a %s card talks about tiling failures:\n%s", name, body)
			}
		})
	}
}
func renderString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func TestTheSheetMeasuresItselfAgainstItsOwnBoxRatherThanTheWindow(t *testing.T) {
	const container = "@container/sheet"
	page := renderString(t, EditCharacter(EditCharacterPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Header:      testCharacterHeader(),
		Derived:     testDerivedValues(),
	}))
	editor := regexp.MustCompile(`<character-editor class="([^"]*)"`).FindStringSubmatch(page)
	if editor == nil {
		t.Fatal("the page renders no character-editor to hang a container on")
	}
	if !strings.Contains(editor[1], container) {
		t.Errorf("character-editor is %q, which names no %s", editor[1], container)
	}
	for name, part := range map[string]templ.Component{
		"the panels": characterPanels(EditCharacterPageData{
			CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			Derived:     testDerivedValues(),
		}),
		"the chips": characterBarChips(testCharacterHeader(), false),
	} {
		rendered := renderString(t, part)
		if at := viewportVariant.FindStringIndex(rendered); at != nil {
			t.Errorf("%s still measure the viewport: %s", name, excerpt(rendered, at[0]))
		}
		if !strings.Contains(rendered, "/sheet:") {
			t.Errorf("%s carry no /sheet variant, so they lay out one way at every width", name)
		}
	}
}

var viewportVariant = regexp.MustCompile(`(^|[^@])max-\[\d+px\]:`)

func excerpt(s string, at int) string {
	start := max(at-40, 0)
	end := min(at+40, len(s))
	return s[start:end]
}
func TestTheLivePanelsRefetchThemselvesOnlyInsideARoom(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const roomID = "01BX5ZZKBKACTAV9WEVGEMMVRZ"
	data := EditCharacterPageData{CharacterID: id, Derived: testDerivedValues()}
	page := renderString(t, characterPanels(data))
	if strings.Contains(page, "room:character") {
		t.Error("the standalone page listens for a socket event it will never hear")
	}
	data.Live = SheetLive{RoomID: roomID}
	window := renderString(t, characterPanels(data))
	for _, section := range LiveSheetSections() {
		want := `hx-get="` + strings.ReplaceAll(SheetSectionPath(roomID, section), "&", "&amp;") + `"`
		if !strings.Contains(window, want) {
			t.Errorf("the %s panel does not refetch itself: no %s", section, want)
		}
	}
	if got := strings.Count(window, "room:character"); got != len(LiveSheetSections()) {
		t.Errorf("%d panels listen for room:character, want %d", got, len(LiveSheetSections()))
	}
	if !strings.Contains(window, "document.activeElement") {
		t.Error("a refetch would swap a panel out from under the caret")
	}
}
func TestASheetSectionIsTheSamePanelThePageAlreadyRenders(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const roomID = "01BX5ZZKBKACTAV9WEVGEMMVRZ"
	data := EditCharacterPageData{CharacterID: id, Derived: testDerivedValues(), Live: SheetLive{RoomID: roomID}}
	page := renderString(t, characterPanels(data))
	for _, section := range LiveSheetSections() {
		fragment := strings.TrimSpace(renderString(t, CharacterSheetSection(data, section)))
		if fragment == "" {
			t.Fatalf("the %s section rendered nothing", section)
		}
		if !strings.Contains(page, fragment) {
			t.Errorf("the %s section is not the markup the page carries", section)
		}
		for _, other := range LiveSheetSections() {
			if other != section && strings.Contains(fragment, panelFormID(other)) {
				t.Errorf("the %s section also carries the %s panel", section, other)
			}
		}
	}
}
func TestTheSheetWindowSwapsSectionsInPlace(t *testing.T) {
	const roomID = "01BX5ZZKBKACTAV9WEVGEMMVRZ"
	window := renderString(t, CharacterSheetWindow(SheetWindowData{
		RoomID:  roomID,
		Section: SheetSectionSpells,
		Level:   3,
	}))
	for _, section := range []string{SheetSectionMain, SheetSectionInventory, SheetSectionSpells} {
		want := `hx-get="` + strings.ReplaceAll(SheetSectionPath(roomID, section), "&", "&amp;") + `"`
		if !strings.Contains(window, want) {
			t.Errorf("the window offers no way back to %s: no %s", section, want)
		}
	}
	for level := 0; level <= MaxSpellLevel; level++ {
		want := strings.ReplaceAll(SheetLevelPath(roomID, level), "&", "&amp;")
		if !strings.Contains(window, want) {
			t.Errorf("spell level %d is not reachable inside the window", level)
		}
	}
	if got := strings.Count(window, `hx-target="#`+SheetBodyID+`"`); got < 3+MaxSpellLevel+1 {
		t.Errorf("%d controls swap the window body; every tab and every level must", got)
	}
	if strings.Contains(window, `href="/characters/`) {
		t.Error("a tab inside the window navigates the whole page away from the room")
	}
}
func TestTheAutosaveGuardStopsTheBrowserSubmitting(t *testing.T) {
	handler, ok := autosaves["hx-on:submit"].(string)
	if !ok || !strings.Contains(handler, "preventDefault") {
		t.Fatalf("autosaves is %v, which does not stop a native submit", autosaves)
	}
	pawn := RoomPawnData{RoomID: "01ROOM", CanEdit: true, IsGM: true, Pawn: RoomPawn{ID: "01PAWN", Name: "Ilyana"}}
	for name, part := range map[string]templ.Component{
		"a saving panel":    savingPanel("Vitals", "/characters/x/vitals", "vitals"),
		"the details panel": RoomPawnFragment(pawn),
		"the grid window":   RoomGrid(RoomGridData{RoomID: "01ROOM"}),
		"the table options": RoomSettings(RoomSettingsData{RoomID: "01ROOM"}),
		"an inventory row":  InventoryRow("01CHAR", InventoryItem{ID: "01ITEM"}),
		"a spell row":       SpellRow("01CHAR", Spell{ID: "01SPELL"}),
		"an attack row":     AttackRow("01CHAR", Attack{ID: "01ATK"}),
	} {
		if rendered := renderString(t, part); !strings.Contains(rendered, `hx-on:submit="event.preventDefault()"`) {
			t.Errorf("%s renders without the guard, so Enter in one of its fields reloads the app", name)
		}
	}
}
func TestAnAutosavingFormCannotSubmitItselfAway(t *testing.T) {
	files, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("read no templates at all")
	}
	opens := regexp.MustCompile(`(?s)<form\b[^>]*>`)
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range opens.FindAllString(string(src), -1) {
			if !strings.Contains(tag, "hx-trigger") || strings.Contains(tag, "submit") {
				continue
			}
			if strings.Contains(tag, "{ autosaves... }") {
				continue
			}
			t.Errorf("%s has a form htmx drives on something other than submit and does "+
				"not spread autosaves, so Enter in one of its fields navigates the page "+
				"away:\n%s", file, tag)
		}
	}
}
func TestNoPanelCanBeMadeUnsaveableByAnEmptyOptionalField(t *testing.T) {
	sparse := EditCharacterPageData{
		CharacterID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Name:        "Vex",
		Size:        DefaultSize,
		Alignment:   DefaultAlignment,
		XP:          "0",
		AC:          "15",
		MaxHP:       "24",
		CurrentHP:   "24",
		TempHP:      "0",
		Str:         "10",
		Dex:         "10",
		Con:         "10",
		Int:         "10",
		Wis:         "10",
		Cha:         "10",
		Derived:     testDerivedValues(),
	}
	rendered := renderString(t, characterPanels(sparse))
	inputs := regexp.MustCompile(`<input[^>]*>`)
	named := regexp.MustCompile(`name="([^"]*)"`)
	valued := regexp.MustCompile(`value="([^"]*)"`)
	for _, tag := range inputs.FindAllString(rendered, -1) {
		if !strings.Contains(tag, " required") {
			continue
		}
		if strings.Contains(tag, `type="checkbox"`) {
			continue
		}
		value := valued.FindStringSubmatch(tag)
		if value != nil && value[1] != "" {
			continue
		}
		field := "an unnamed field"
		if match := named.FindStringSubmatch(tag); match != nil {
			field = match[1]
		}
		t.Errorf("%s is required and renders empty, so htmx refuses to post the whole "+
			"panel and form-validity.js swallows the reason: the panel saves nothing, silently", field)
	}
}

const sheetCharacterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

var sheetHeadings = regexp.MustCompile(`<h2 class="m-0 font-serif text-\[0\.82rem\][^"]*">([^<]*)</h2>`)

func panelOrder(markup string) []string {
	var out []string
	for _, match := range sheetHeadings.FindAllStringSubmatch(markup, -1) {
		out = append(out, strings.ReplaceAll(match[1], "&amp;", "&"))
	}
	return out
}
func testSheetPage() EditCharacterPageData {
	return EditCharacterPageData{
		CharacterID: sheetCharacterID,
		Attacks:     []Attack{{ID: testAttackRowID, Name: "Longsword"}},
		Features:    []Feature{{Name: "Second Wind"}},
	}
}
func TestTheSheetWindowOpensOnWhatIsUsedInPlay(t *testing.T) {
	want := []string{
		"Vitals",
		"Saving Throws",
		"Abilities",
		"Skills",
		"Attacks",
		"Prepared Spells",
		"Spell Slots",
		"Equipment",
		"Features & Traits",
		"Identity",
		"Core Stats",
		"Proficiencies & Training",
		"Personality",
		"Appearance",
	}
	var buf bytes.Buffer
	data := SheetWindowData{Section: SheetSectionMain, Sheet: testSheetPage()}
	if err := CharacterSheetWindow(data).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := panelOrder(buf.String())
	if len(got) != len(want) {
		t.Fatalf("panels = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("panel %d = %q, want %q (order = %v)", i, got[i], name, got)
		}
	}
}
func TestTheSheetWindowAndTheEditorHoldTheSamePanels(t *testing.T) {
	data := testSheetPage()
	var page, window bytes.Buffer
	if err := EditCharacter(data).Render(context.Background(), &page); err != nil {
		t.Fatalf("render page: %v", err)
	}
	if err := CharacterSheetWindow(SheetWindowData{Section: SheetSectionMain, Sheet: data}).Render(context.Background(), &window); err != nil {
		t.Fatalf("render window: %v", err)
	}
	onPage := panelOrder(page.String())
	inWindow := panelOrder(window.String())
	slices.Sort(onPage)
	slices.Sort(inWindow)
	if !slices.Equal(onPage, inWindow) {
		t.Errorf("the two arrangements no longer hold the same panels:\n  page   = %v\n  window = %v", onPage, inWindow)
	}
}

var controlClass = regexp.MustCompile(`class="(input|select|textarea|checkbox)\b([^"]*)"`)

func TestEveryControlOnTheSheetShrinksWithIt(t *testing.T) {
	pages := map[string]templ.Component{
		"character": CharacterSheetWindow(SheetWindowData{Section: SheetSectionMain, Sheet: testSheetPage()}),
		"inventory": CharacterSheetWindow(SheetWindowData{
			Section:   SheetSectionInventory,
			Inventory: InventoryPageData{CharacterID: sheetCharacterID, Items: []InventoryItem{{ID: testItemID}}},
		}),
		"spells": CharacterSheetWindow(SheetWindowData{
			Section: SheetSectionSpells,
			Level:   3,
			Spells: SpellLevelPageData{
				CharacterID: sheetCharacterID,
				Level:       3,
				Current:     testSpellCounters(3),
				Spells:      []Spell{{ID: testSpellID}},
			},
		}),
	}
	for name, page := range pages {
		var buf bytes.Buffer
		if err := page.Render(context.Background(), &buf); err != nil {
			t.Fatalf("render %s: %v", name, err)
		}
		markup := buf.String()
		controls := controlClass.FindAllStringSubmatch(markup, -1)
		if len(controls) == 0 {
			t.Fatalf("%s: found no controls at all, so this proves nothing", name)
		}
		for _, match := range controls {
			if strings.Contains(match[2], "-sm") || strings.Contains(match[2], "-xs") {
				continue
			}
			t.Errorf("%s: a %s on the sheet keeps its page-sized control: %q", name, match[1], match[0])
		}
		legends := strings.Count(markup, `class="fieldset-legend`)
		dense := strings.Count(markup, `class="fieldset-legend `+denseLegend+`"`)
		if legends != dense {
			t.Errorf("%s: %d of %d field labels do not shrink with the sheet", name, legends-dense, legends)
		}
	}
}

func TestPanelsFlattenInTheWindowAndFloatOnThePage(t *testing.T) {
	data := testSheetPage()
	window := renderString(t, CharacterSheetWindow(SheetWindowData{Section: SheetSectionMain, Sheet: data}))
	if !strings.Contains(window, "@container/window") {
		t.Errorf("the sheet window names no container, so the flat variants below never bite\n%s", window)
	}
	if !strings.Contains(window, flatInWindow) {
		t.Errorf("the panels in the window keep their background and shadow\n%s", window)
	}
	page := renderString(t, EditCharacter(data))
	if strings.Contains(page, "@container/window") {
		t.Error("the full page names a window container, so its panels flatten too")
	}
	if !strings.Contains(page, "shadow-panel") {
		t.Error("the full page lost the floating panels the window does not want")
	}
}
