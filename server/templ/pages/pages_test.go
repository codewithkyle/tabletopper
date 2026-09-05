package pages

import (
	"bytes"
	"context"
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
			return render(EditCharacter(EditCharacterPageData{SpellLevels: EmptySpellLevels()}))
		},
		"edit-character-spells": func() error {
			return render(EditCharacterSpells(EditCharacterPageData{SpellLevels: EmptySpellLevels()}))
		},
		"edit-character-inventory": func() error { return render(EditCharacterInventory(InventoryPageData{})) },
		"assets":                   func() error { return render(MapAssets([]queries.Asset{})) },
		"sign-in":                  func() error { return render(SignIn(ClerkFrontend{})) },
		"tos":                      func() error { return render(TOS()) },
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

// The editor is two pages. Each renders one form per panel it carries, posting
// to its own route as the user types -- so this pins the routes, the debounce,
// the split itself, and the absence of anything to press.
func TestEditCharacterRendersOneFormPerPanel(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	base := "/characters/" + id

	for _, page := range []struct {
		name    string
		render  func(EditCharacterPageData) templ.Component
		panels  map[string]string
		absent  string
		current string
	}{
		{
			name:   "character",
			render: EditCharacter,
			panels: map[string]string{
				"identity":      base + "/identity",
				"abilities":     base + "/abilities",
				"core-stats":    base + "/core-stats",
				"proficiencies": base + "/proficiencies",
				"saving_throws": base + "/bonuses/saving_throws",
				"skills":        base + "/bonuses/skills",
				"features":      base + "/features",
			},
			absent:  base + "/spells",
			current: base + "/edit",
		},
		{
			name:    "spells",
			render:  EditCharacterSpells,
			panels:  map[string]string{"spells": base + "/spells"},
			absent:  base + "/identity",
			current: base + "/edit/spells",
		},
	} {
		t.Run(page.name, func(t *testing.T) {
			var buf bytes.Buffer
			data := EditCharacterPageData{CharacterID: id, SpellLevels: EmptySpellLevels()}
			if err := page.render(data).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			markup := buf.String()

			for panel, action := range page.panels {
				if want := `hx-post="` + action + `"`; !strings.Contains(markup, want) {
					t.Errorf("no panel posts to %s", action)
				}
				if want := `id="errors-` + panel + `"`; !strings.Contains(markup, want) {
					t.Errorf("panel %q has no error block to swap into", panel)
				}
			}

			// The split is the point: a panel belonging to the other page must
			// not be here, or both pages would write the same columns.
			if strings.Contains(markup, `hx-post="`+page.absent+`"`) {
				t.Errorf("%s carries a panel that belongs to the other page", page.name)
			}

			if got := strings.Count(markup, "hx-post="); got != len(page.panels) {
				t.Errorf("posting forms = %d, want %d (one per panel, and none around them)", got, len(page.panels))
			}

			// The panels plus Base's own are every form on the page, so an
			// extra one would mean something still wraps the sheet.
			if got := strings.Count(markup, "<form"); got != len(page.panels)+closingForms {
				t.Errorf("forms = %d, want %d", got, len(page.panels)+closingForms)
			}

			// The debounce is what makes typing one save rather than one per
			// keystroke.
			if got := strings.Count(markup, `hx-trigger="input delay:1s, repeater:changed"`); got != len(page.panels) {
				t.Errorf("debounced panels = %d, want %d", got, len(page.panels))
			}

			// Nothing to press: the panels save themselves.
			if strings.Contains(markup, `type="submit"`) {
				t.Error("the editor still renders a submit button")
			}

			// Every tab is reachable from every page, and exactly the one you
			// are on is marked current.
			for _, href := range []string{base + "/edit", base + "/edit/inventory", base + "/edit/spells"} {
				if !strings.Contains(markup, `href="`+href+`"`) {
					t.Errorf("no way to reach %s from here", href)
				}
			}
			if got := strings.Count(markup, `aria-current="page"`); got != 1 {
				t.Errorf("current tabs = %d, want exactly 1", got)
			}
			// Matched on the attribute rather than the class string, so
			// restyling the links does not break the test. The closing quote is
			// load-bearing: the Character href is a prefix of the other two.
			if want := `href="` + page.current + `" aria-current="page"`; !strings.Contains(markup, want) {
				t.Errorf("the current tab is not %s", page.current)
			}
		})
	}
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
		SpellLevels: EmptySpellLevels(),
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
