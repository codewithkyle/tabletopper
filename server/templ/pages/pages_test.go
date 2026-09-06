package pages

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/a-h/templ"
)

// Base renders three modal dialogs holding four method="dialog" forms between
// them: one each in the alert and confirm dialogs, and two in the content modal,
// whose loading and error states carry a Close apiece now that the shell has no
// corner ✕. Every page carries these, so a page's own forms are its total minus
// this.
const closingForms = 4

// Every page used to assign into a package-level settings struct on each
// render, which the race detector flags the moment two renders overlap. The
// layout now takes its title and body per call; this keeps it that way.
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
		"journal-link-fragment": func() error { return render(JournalLinkFragment()) },
		"journal-entries-fragment": func() error {
			return render(JournalEntriesFragment(JournalPageData{Entries: []JournalEntry{testJournalEntry()}}))
		},
		"assets":  func() error { return render(MapAssets([]queries.Asset{})) },
		"sign-in": func() error { return render(SignIn(ClerkFrontend{})) },
		"tos":     func() error { return render(TOS()) },
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

// The Character tab renders one form per panel, each posting to its own route as
// the user types -- so this pins the routes, the debounce, and the absence of
// anything to press. It used to run over both editor pages; the spells tab is
// not a panel page any more and has tests of its own below.
func TestEditCharacterRendersOneFormPerPanel(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	base := "/characters/" + id

	panels := map[string]string{
		"identity":      base + "/identity",
		"abilities":     base + "/abilities",
		"core-stats":    base + "/core-stats",
		"proficiencies": base + "/proficiencies",
		"saving_throws": base + "/bonuses/saving_throws",
		"skills":        base + "/bonuses/skills",
		"features":      base + "/features",
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

	// The split is the point: spellcasting belongs to the other tab, and a
	// panel here would write columns that page also writes.
	for _, absent := range []string{base + "/spells", base + "/spells/slots/1"} {
		if strings.Contains(markup, `hx-post="`+absent+`"`) {
			t.Errorf("the Character tab carries %s, which belongs to the spells pages", absent)
		}
	}

	if got := strings.Count(markup, "hx-post="); got != len(panels) {
		t.Errorf("posting forms = %d, want %d (one per panel, and none around them)", got, len(panels))
	}

	// The panels plus Base's own are every form on the page, so an extra one
	// would mean something still wraps the sheet.
	if got := strings.Count(markup, "<form"); got != len(panels)+closingForms {
		t.Errorf("forms = %d, want %d", got, len(panels)+closingForms)
	}

	// The debounce is what makes typing one save rather than one per keystroke.
	if got := strings.Count(markup, `hx-trigger="input delay:1s, repeater:changed"`); got != len(panels) {
		t.Errorf("debounced panels = %d, want %d", got, len(panels))
	}

	// Nothing to press: the panels save themselves.
	if strings.Contains(markup, `type="submit"`) {
		t.Error("the editor still renders a submit button")
	}

	assertCharacterTabs(t, markup, base+"/edit")
}

// Every tab is reachable from every editor page, and the one you are on is
// marked current. The spells pages carry a second nav with a current link of
// their own, so this checks the links it expects rather than counting
// attributes.
//
// The Spells tab points at cantrips, not at a bare /edit/spells. There is no
// index above the levels, and a tab aimed at one would 302 on every click.
func assertCharacterTabs(t *testing.T, markup string, current string) {
	t.Helper()

	base := strings.TrimSuffix(current, "/edit")
	base = strings.SplitN(base, "/edit/", 2)[0]

	for _, href := range []string{base + "/edit", base + "/edit/inventory", base + "/edit/spells/0", base + "/edit/journal"} {
		if !strings.Contains(markup, `href="`+href+`"`) {
			t.Errorf("no way to reach %s from here", href)
		}
	}

	// Matched on the attribute rather than the class string, so restyling the
	// links does not break the test. The closing quote is load-bearing: the
	// Character href is a prefix of the other two.
	if want := `href="` + current + `" aria-current="page"`; !strings.Contains(markup, want) {
		t.Errorf("the current tab is not %s", current)
	}
}

// testSpellCounters is one level's slot counters, which is all a spells page
// carries about levels now.
func testSpellCounters(level int) SpellLevel {
	return SpellLevel{Level: level, Slots: "0", Used: "0"}
}

// The new-character dialog. Its three targeting attributes have to agree with
// the id of the block they aim at, and a disagreement is invisible: the reply
// lands nowhere and the dialog sits there looking like the button is broken. So
// the exact string is pinned rather than its shape.
//
// The 422 override is pinned for the same reason at one remove. base.templ puts
// the whole 4xx range in noSwap, so without it a rejected name would replace
// nothing -- the form would post, the server would answer, and the user would
// see no difference.
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

	// The dialog carries its own way out. The shell has no corner control to
	// fall back on, so a fragment that forgets this leaves Escape as the only
	// exit -- which looks like a dialog that will not close.
	if !strings.Contains(body, "modal:close") {
		t.Errorf("fragment has no Close button\n%s", body)
	}

	// A name needs no worked example, and one in the box reads as a value that
	// is already there.
	if strings.Contains(body, "placeholder=") {
		t.Errorf("the name field has a placeholder\n%s", body)
	}

	// One field, deliberately. The dialog asks for a name and sends the user to
	// a page built to hold everything else; a second control here is the start
	// of rebuilding the create page inside a 24rem box.
	if inputs := strings.Count(body, "<input"); inputs != 1 {
		t.Errorf("fragment has %d inputs, want 1", inputs)
	}
}

const (
	testItemID    = "01BX5ZZKBKACTAV9WEVGEMMVS0"
	testItemPanel = "errors-inventory-" + testItemID
)

// The inventory row, which is the only saving thing on the sheet that is a form
// per row rather than a form per panel. Its three targeting attributes have to
// agree with the id of the block they aim at, and a disagreement is silent: the
// reply lands nowhere and the row looks like it is not saving. So the exact
// strings are pinned, the way the new-character dialog's are.
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

	// Nothing to press. The row saves itself, and the one button on it deletes.
	if strings.Contains(body, `type="submit"`) {
		t.Error("the row renders a submit button")
	}
}

// ALL SIX CONTROLS, ALWAYS. buildInventoryInput reads `equipped` from whether the
// field arrived, because an unchecked box posts nothing -- so a post without it
// means unticked. That is only true while the row renders every control on every
// render: a variant that dropped the checkbox would silently unequip an item on
// its next autosave, and nothing on the server could tell.
//
// THE DISCLOSURE IS WHY THIS NEEDS SAYING TWICE. The checkbox and the
// description live inside a <details>, which is CLOSED and not ABSENT -- a
// collapsed <details> keeps its contents in the DOM and the form still submits
// them. A row that rendered its details only when open would unequip every item
// on the first keystroke after a page load.
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

// Field names carry no row prefix -- the form is the row, so a post contains one
// item's fields and nothing else. That is what lets the handler read plain
// `name` and `quantity`, and it only holds because forms do not nest: the panel
// around these is a plain card and not a savingPanel.
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
	// The item forms plus Base's own closing forms, and nothing wrapping them.
	if got := strings.Count(body, "<form"); got != len(data.Items)+closingForms {
		t.Errorf("forms = %d, want %d", got, len(data.Items)+closingForms)
	}

	// The add button posts to the collection and appends what comes back. It is
	// outside every form, which is what keeps it from being serialised into one.
	if want := `hx-post="/characters/` + characterID + `/inventory"`; !strings.Contains(body, want) {
		t.Errorf("no add button posting to %s", want)
	}
	if !strings.Contains(body, `hx-swap="append"`) {
		t.Error("the add button does not append its reply")
	}
}

// Equipment on the Character page is a view of rows the inventory table owns. It
// must not become a form again: the page's form count is asserted above as one
// per saving panel, and a control here would post to a route that no longer
// exists -- the weapons column it used to write is gone.
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

	// A quantity of one is the default and says nothing, so it is not printed.
	// Four javelins is worth knowing.
	if strings.Contains(body, "&#215; 1<") {
		t.Error("a quantity of 1 is printed beside an item")
	}
	if !strings.Contains(body, "&#215; 4") {
		t.Errorf("a quantity above 1 is not printed\n%s", body)
	}

	// A row can be ticked before it is named, and an empty entry on the sheet
	// reads as a rendering fault rather than an unfinished row.
	if !strings.Contains(body, "Unnamed item") {
		t.Errorf("an unnamed equipped row renders as nothing\n%s", body)
	}
}

// With nothing ticked the panel has to say where ticking happens, or it is an
// empty box on a page that gives no hint the inventory page exists.
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

// The two repeaters that inventory replaced are gone from the page, not merely
// unreferenced. Both wrote columns that no longer exist, so a panel left behind
// would post to a handler that answers 404 and look like a save that never
// lands.
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

	// And the replacement is actually wired up, not just absent.
	if !strings.Contains(body, "Chain Mail") {
		t.Errorf("the equipped rows are not rendered on the page\n%s", body)
	}
}

const (
	testSpellID    = "01BX5ZZKBKACTAV9WEVGEMMVS0"
	testSpellPanel = "errors-spell-" + testSpellID
)

// The spell row, which is the inventory row's shape at a different table. Its
// three targeting attributes have to agree with the id of the block they aim at,
// and a disagreement is silent: the reply lands nowhere and the row looks like
// it is not saving. So the exact strings are pinned.
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

	// Nothing to press. The row saves itself, and the one button on it deletes.
	if strings.Contains(body, `type="submit"`) {
		t.Error("the row renders a submit button")
	}

	// The level is in the URL and nowhere else. A spell cannot move between
	// levels, so UpdateSpell does not name the column and no control offers it.
	if strings.Contains(body, `name="level"`) {
		t.Errorf("the row renders a level control\n%s", body)
	}
}

// ALL EIGHT CONTROLS, ALWAYS. buildSpellInput reads `prepared` from whether the
// field arrived, because an unchecked box posts nothing -- so a post without it
// means unticked. That is only true while the row renders every control on every
// render: a variant that dropped the checkbox would silently unprepare a spell
// on its next autosave, and nothing on the server could tell.
//
// The disclosure is why this needs saying twice. Five of the eight sit inside a
// <details>, which is closed for a named spell -- closed, not absent. A row that
// rendered its details only when open would post five empty strings on every
// save and wipe the spell.
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

// A row arrives from the add button with nothing in it, and the five fields
// worth filling in are behind the disclosure. So a nameless row opens itself and
// a named one stays shut, which is the whole of the density fix: the level pages
// split ten sections into ten pages, and this is what keeps one page from being
// eight tall cards.
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

// THERE IS NO SPELLS INDEX. The tab opens on cantrips, which is where a level-1
// caster's entire spell list lives, and the level strip is how you reach
// anything else. A link to the bare /edit/spells would be a redirect on every
// click, so nothing renders one.
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

			// The closing quote is load-bearing: every level href starts with
			// this string.
			if strings.Contains(body, `href="/characters/`+characterID+`/edit/spells"`) {
				t.Errorf("%s links to a spells index\n%s", page.name, body)
			}
			if want := `href="/characters/` + characterID + `/edit/spells/0"`; !strings.Contains(body, want) {
				t.Errorf("%s has no way into the spells tab", page.name)
			}
		})
	}
}

// The level strip is ten tabs and nothing else. Every level is one click away
// from every other, which is what a stack of ten sections was trying to be.
//
// THE TABS ARE STATIC LABELS. They carry a level and a link and no data, which
// is why spellLevelTabs takes no slice: nothing about a level the page is not
// showing has to be read to render them.
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

	// No count badge. The page renders two spells at this level and the tab
	// says nothing about them -- the label is the level and only the level.
	tabs := body[strings.Index(body, `aria-label="Spell levels"`):]
	tabs = tabs[:strings.Index(tabs, "</nav>")]
	for _, digit := range []string{">2<", ">0<"} {
		if strings.Contains(tabs, digit) {
			t.Errorf("the level tabs carry a count: %s\n%s", digit, tabs)
		}
	}
	// The labels themselves survive that check because none of them is a bare
	// digit -- 3rd, not 3.
	for _, want := range []string{">Cantrips<", ">1st<", ">3rd<", ">9th<"} {
		if !strings.Contains(tabs, want) {
			t.Errorf("the level tabs are missing %s\n%s", want, tabs)
		}
	}
}

// A level page carries its own counters, its own rows and an add button aimed at
// its own collection. Getting the level wrong in any of the three would write to
// a level the user is not looking at.
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

	// Two spell rows plus this level's counters, and nothing wrapping them.
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

	// No other level's counters are reachable from here -- that is what the
	// overview is for, and a stray form would write a level off screen.
	for _, level := range []string{"1", "2", "4", "9"} {
		if strings.Contains(body, "/spells/slots/"+level+`"`) {
			t.Errorf("the level 3 page carries level %s's counters", level)
		}
	}

	// Both navs mark where you are: the character tabs say Spells, the level
	// tabs say which level.
	assertCharacterTabs(t, body, "/characters/"+characterID+"/edit/spells/0")
	if want := `href="/characters/` + characterID + `/edit/spells/3" aria-current="page"`; !strings.Contains(body, want) {
		t.Errorf("the level tabs do not mark level 3 as current\n%s", body)
	}
}

// Cantrips are a level page with no counters, because they have no slots. The
// route that would write them refuses level 0 as well; this is the half of that
// which the user can see.
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

// Prepared Spells on the Character page is a view of rows the spells table owns,
// exactly as Equipment is a view of the inventory rows. It must not become a
// form: the page's form count is asserted above as one per saving panel, and a
// control here would post to a route that expects a level in its path.
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

	// The meta line is the three things worth knowing before casting. The spell
	// text is not among them -- ten paragraphs here would be the wall the level
	// pages exist to remove.
	if !strings.Contains(body, "Action \u00b7 150 feet \u00b7 Instantaneous") {
		t.Errorf("the meta line is not rendered\n%s", body)
	}

	// A spell can be ticked before it is named, and an empty entry on the sheet
	// reads as a rendering fault rather than an unfinished row.
	if !strings.Contains(body, "Unnamed spell") {
		t.Errorf("an unnamed prepared row renders as nothing\n%s", body)
	}
}

// With nothing prepared the panel has to say where preparing happens, or it is
// an empty box on a page that gives no hint the spells pages exist.
func TestPreparedSpellsEmptyStatePointsAtTheSpellsPage(t *testing.T) {
	const characterID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	var buf bytes.Buffer
	if err := preparedSpells(characterID, nil).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()

	// Cantrips, not a bare /edit/spells -- there is no index above the levels,
	// and pointing an empty state at a redirect is a click nobody needs.
	if want := `href="/characters/` + characterID + `/edit/spells/0"`; !strings.Contains(body, want) {
		t.Errorf("the empty state does not point at %s\n%s", want, body)
	}
	if !strings.Contains(body, "Prepared") {
		t.Errorf("the empty state does not name the control that fills it\n%s", body)
	}
}

// A spell with nothing filled in but its name gets a name and no separator, not
// a line of stray middle dots.
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
		if got := spellMetaLine(c.spell); got != c.want {
			t.Errorf("meta line = %q, want %q", got, c.want)
		}
	}
}

// Both read-only views are on the Character page and wired to real data, not
// merely present. Each is the only place its table's rows surface outside the
// tab that owns them.
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

	// Prepared Spells sits in its own row below the four panels above it, beside
	// Spell Slots -- not stacked under Equipment, where a list that can run to
	// twenty spells pushed the right column past the left one.
	prepared := strings.Index(body, "Prepared Spells")
	equipment := strings.Index(body, "Equipment")
	slots := strings.Index(body, "Spell Slots")
	if !(equipment < prepared && prepared < slots) {
		t.Errorf("panel order is Equipment %d, Prepared %d, Slots %d", equipment, prepared, slots)
	}
}

// testSpellLevels is the ten-level summary the Spell Slots panel is handed,
// which the controller builds from however few rows the two queries returned.
func testSpellLevels() []SpellLevel {
	levels := make([]SpellLevel, 0, MaxSpellLevel+1)
	for level := 0; level <= MaxSpellLevel; level++ {
		levels = append(levels, SpellLevel{Level: level, Slots: "0", Used: "0"})
	}

	return levels
}

// The Spell Slots panel is one little form per level in use, sitting on the
// Character tab beside Prepared Spells. It is the one editable thing on that
// page that does not write a characters column, and the reason it is there is
// the long rest: resetting `used` across several levels is one screen here and
// one page load each on the level pages.
//
// LEVELS THE CHARACTER HAS NOTHING AT ARE NOT HERE. A level with no slots and no
// spells is a row of zeroes and two inputs nobody will touch; it comes back from
// its own page, where the counters always render.
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

	// Three forms, not four: cantrips are in use but have no slots in the rules,
	// so level 0 gets a link and the word Unlimited where the counters would be.
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

	// Seven saving panels plus three slot forms plus Base's own, and nothing
	// wrapping them. Forms do not nest, so a slot form inside a savingPanel
	// would post neither.
	const panels = 7
	if got := strings.Count(body, "<form"); got != panels+3+closingForms {
		t.Errorf("forms = %d, want %d", got, panels+3+closingForms)
	}

	// Every level in use links to its own page, so the panel is also how you get
	// to a level without going through the tab strip.
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

	// The count reads as a sentence, because it sits next to two counters that
	// are also numbers. A level that is here for its slots alone still says it
	// holds nothing.
	for _, want := range []string{"5 spells", "2 spells", "No spells"} {
		if !strings.Contains(body, want) {
			t.Errorf("the panel is missing %q", want)
		}
	}
}

// Which levels are in use is a display decision and nothing else reads it, so it
// is worth pinning on its own rather than only through a rendered page.
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
		// used without slots cannot be written -- SaveSpellSlots caps used at
		// slots -- but if it ever were, hiding the level would strand it.
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

// With every level empty there is no list at all, only a line saying where the
// first one comes from -- otherwise a fighter's sheet carries a blank box.
//
// Cantrips are the case worth its own row here: a character whose only
// spellcasting is a cantrip has a list with no form in it, because level 0 has
// no counters. Counting forms would call that an empty panel. So the empty state
// is the thing asserted, and the levels linked are asserted beside it -- the
// empty state carries a link of its own, and matching on hrefs alone would call
// it a list.
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
				// The empty state points at cantrips, which is the way to the
				// first slot count -- so level 0 is expected there too.
				want := shown[level] || (empty && level == 0)
				if got := strings.Contains(body, href); got != want {
					t.Errorf("level %d linked = %v, want %v\n%s", level, got, want, body)
				}
			}
		})
	}
}

// A prepared spell shows the start of its text, clamped to two lines, and
// expands in place to the rest.
//
// The clamp is visual only -- the whole description is in the DOM either way --
// so expanding costs no request, and a spell whose text runs to a paragraph does
// not push the next one off the panel. It is a <details>, not a title attribute,
// because this sheet gets read on a tablet at a table and a native tooltip has
// nothing to hover.
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

	// One disclosure, for the one spell that has text. A spell with none gets no
	// control at all rather than an empty one that opens onto nothing.
	if got := strings.Count(body, "<details"); got != 1 {
		t.Errorf("disclosures = %d, want 1 -- only Fireball has text", got)
	}

	for _, want := range []string{"line-clamp-2", "group-open:line-clamp-none", "cursor-pointer"} {
		if !strings.Contains(body, want) {
			t.Errorf("the description is missing %s\n%s", want, body)
		}
	}

	// THE WHOLE TEXT IS THERE, not an excerpt cut in Go. The two-line limit is
	// CSS, so expanding shows the rest without asking the server for it -- and a
	// truncation done here would have made the expansion a lie.
	if !strings.Contains(body, long) {
		t.Errorf("the description was truncated before it reached the markup\n%s", body)
	}

	// Newlines in a spell's text survive, the way they do on the inventory rows.
	if !strings.Contains(body, "whitespace-pre-line") {
		t.Errorf("the description collapses its line breaks\n%s", body)
	}
}

// A row arrives from the add button with nothing in it, so it opens its own
// disclosure -- the spell rows do the same, and adding armour usually means
// ticking Equipped in the same breath. A row that already has a name stays shut,
// which is the whole point of the collapse.
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

// testJournalEntry is one row of the journal list, with the two dates the
// controller has already rendered.
func testJournalEntry() JournalEntry {
	return JournalEntry{
		ID:      testEntryID,
		Title:   "Session 12",
		Created: Timestamp{ISO: "2026-09-05T18:04:11Z", Text: "5 Sep 2026, 18:04 UTC"},
		Updated: Timestamp{ISO: "2026-09-06T09:30:00Z", Text: "6 Sep 2026, 09:30 UTC"},
	}
}

const testEntryID = "01BX5ZZKBKACTAV9WEVGEMMVS1"

// The journal entry page is a panel like any other: it posts itself on a
// debounce and swaps the reply into its own error block. Its three targeting
// attributes have to agree with the id of the block they aim at, and a
// disagreement is silent -- the reply lands nowhere and the editor looks like it
// is not saving -- so the exact strings are pinned, the way the inventory row's
// are.
//
// THE POST GOES TO THE RESOURCE URL, NOT THE PAGE URL. The page is under /edit/
// and the mutation is not; posting to the page would 404 on a route that only
// answers GET.
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
		`<div id="errors-journal"></div>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}

	// The stored markdown reaches the browser inside the textarea and nowhere
	// else. journal-editor.js reads its initial content from there, because
	// templ escapes the text of a textarea -- an entry inlined into a <script>
	// block would be a script-injection vector instead.
	if !strings.Contains(markup, `We went back to the marsh.</textarea>`) {
		t.Errorf("the body is not in the textarea\n%s", markup)
	}
	if !strings.Contains(markup, `name="body"`) || !strings.Contains(markup, `name="title"`) {
		t.Errorf("the form does not carry both fields\n%s", markup)
	}

	assertCharacterTabs(t, markup, "/characters/"+characterID+"/edit/journal")
}

// The Save button is not a second save path. It posts the SAME form to the SAME
// route as the debounce, from outside the form -- which is what hx-include is
// for, and why savingPanel gives every panel form an id. Getting that selector
// wrong is the dangerous failure here: the request would go out with no title
// and no body and blank the entry, so the id and the include are pinned
// together.
//
// It exists because the autosave is deliberately silent (see
// finishJournalEntry), and `announce` is what asks the server to say so. The
// debounce never sends that field.
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

	// Beside Back, in the page header, and after it -- neutral first and the
	// affirmative action second, the order every dialog in the app uses.
	back := strings.Index(markup, ">Back</a>")
	save := strings.Index(markup, ">Save</button>")
	if back < 0 || save < back {
		t.Errorf("Save is not beside Back in the header\n%s", markup)
	}
}

// EVERY TOOLBAR BUTTON IS type="button". The toolbar sits inside the autosaving
// form, and a bare <button> there is a submit -- so one missing attribute turns
// "make this bold" into a full-page post to the save route.
//
// The editor is additive: the toolbar starts hidden and the textarea starts
// visible, and journal-editor.js swaps the two once it has an editor to drive.
// With the module absent or still loading, the entry is editable as plain
// markdown rather than not editable at all.
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

	// The heading control carries no name, so the form does not post it at all
	// and the save handler has nothing to ignore.
	if strings.Contains(markup, `data-journal-heading name=`) || strings.Contains(markup, `name="heading"`) {
		t.Errorf("the heading select is posted with the form\n%s", markup)
	}
}

// THE UPLOAD URL IS THE SAVE URL WITH /images ON THE END, and it is rendered
// once, here, onto the editor root. journal-editor.js reads it off the dataset
// rather than building it from ids of its own, so this attribute is the whole
// contract: lose it and every paste posts to undefined.
//
// The button is data-journal-upload and NOT data-journal-mark. The editor sets
// a pressed state on everything carrying the latter, and an upload button has
// none to report -- the count above is 6 for that reason and would be 7 if the
// attribute were shared.
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

	// The button is the affordance; the input is hidden because a bare file
	// input cannot be styled into the toolbar, and it carries the accept list
	// so the picker filters before the server has to refuse.
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

// Close comes first and the affirmative action second, in every dialog in the
// app, and the fragment loaded into the shared modal has to carry its own --
// the shell supplies nothing to what it fetches.
//
// The form has no hx-* of its own on purpose: there is nothing to post. The
// data-journal-link attribute is the whole contract with journal-editor.js,
// which fills the field and takes the submit, so losing it would leave a dialog
// whose Insert button did nothing.
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

// Dates are rendered twice: RFC 3339 in the attribute for the machine, and the
// server's own UTC rendering as the text for the reader.
//
// THE TEXT IS NOT DECORATION. It is what shows before local-time.js runs, with
// JavaScript off, and in a test -- the module rewrites it into the reader's
// locale and zone, which the server cannot do because it holds an instant and
// nothing about where the reader is.
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
		`<local-time><time datetime="2026-09-05T18:04:11Z">5 Sep 2026, 18:04 UTC</time></local-time>`,
		`<local-time><time datetime="2026-09-06T09:30:00Z">6 Sep 2026, 09:30 UTC</time></local-time>`,
	} {
		if !strings.Contains(collapseWhitespace(markup), want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}

	// The card links to the page and deletes through the resource URL. Those are
	// two different paths, and swapping them would either delete nothing or
	// navigate to a route that answers no GET.
	//
	// TWO WAYS IN, and both are the same link: the title, for anyone who reads
	// the list as a list, and a View button beside Delete, for anyone who reads
	// it as a row of controls. A row whose only affordance is a destructive
	// button is a row you can only delete.
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

// Creation is a page-level action, so its button is in the page header beside
// Back rather than at the bottom of the panel -- the panel is a list of what
// exists, and the thing that makes a new one is not part of that list.
//
// IT IS A PLAIN FORM POST with no hx-* at all. There is no field to collect, so
// there is nothing to reject and nothing to keep the page open for: the handler
// inserts a blank entry and 303s into its editor, and the browser follows.
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

	// The panel holds the list and nothing that posts. Bounded at the editor
	// element, because the base layout's three dialogs each carry a
	// method="dialog" form of their own further down the page.
	panel := markup[strings.Index(markup, "journal-entries"):strings.Index(markup, "</character-editor>")]
	if strings.Contains(panel, "<form") {
		t.Errorf("a form survives inside the panel\n%s", panel)
	}
}

// An entry is born blank -- creation takes no fields -- so the list has to say
// something in the space where its title goes.
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

// The box swaps the list on every pause in typing, so it cannot be inside the
// thing it swaps: an input that replaces itself mid-type loses the caret and its
// own value with it, and the reader gets one character per swap. This pins the
// order -- box first, target second -- and pins the target to the id the
// container actually carries, because a target that has drifted fails silently.
// htmx finds nothing to swap and the box just stops working.
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
	// A GET, and under /fragment/, because the reply is part of a page that is
	// already open. The character rides in the query string; htmx appends the
	// box's own q beside it rather than replacing what is already there.
	if !strings.Contains(markup, `hx-get="/fragment/character/journal-entries?character=`+characterID+`"`) {
		t.Errorf("the box does not call the fragment route\n%s", markup)
	}
	// The server refuses a longer term with a 404, which is an empty reply and
	// so a list that silently stops updating. This is what keeps that
	// unreachable from the control that sends it.
	if !strings.Contains(markup, `maxlength="255"`) {
		t.Errorf("the box is not capped at the length the server accepts\n%s", markup)
	}
}

// An empty list has two causes and they need two different sentences. Nothing
// written is a journal to start, and the message points at the button that
// starts one. Nothing matched is a search that missed, and telling that reader
// to go and write something answers a question they did not ask.
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

// A fragment is never a second copy of markup. The search route returns the same
// component the page renders, so a card that grows a control grows it in both
// places at once -- and this fails the moment the two are allowed to drift,
// because the fragment stops being a substring of the page.
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
	// The fragment is what lands inside the container, so it must not bring a
	// second one with it.
	if strings.Contains(fragment.String(), `id="`+journalEntriesID+`"`) {
		t.Errorf("the fragment carries the container it is swapped into\n%s", fragment.String())
	}
}

// collapseWhitespace flattens the indentation templ writes between elements, so
// a test can pin the shape of a nested run of markup as one string.
func collapseWhitespace(markup string) string {
	return strings.Join(strings.Fields(markup), " ")
}
