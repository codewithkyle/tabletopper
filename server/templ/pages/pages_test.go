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
				"features":      base + "/rows/features",
				"weapons":       base + "/rows/weapons",
				"resources":     base + "/rows/resources",
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

			// Base renders three modal dialogs holding four method="dialog"
			// forms between them: one each in the alert and confirm dialogs, and
			// two in the content modal, whose loading and error states carry a
			// Close apiece now that the shell has no corner ✕. Those plus the
			// panels are every form on the page, so an extra one would mean
			// something still wraps the sheet.
			const closingForms = 4
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

			// Both tabs are reachable from either page, and exactly the one you
			// are on is marked current.
			for _, href := range []string{base + "/edit", base + "/edit/spells"} {
				if !strings.Contains(markup, `href="`+href+`"`) {
					t.Errorf("no way to reach %s from here", href)
				}
			}
			if got := strings.Count(markup, `aria-current="page"`); got != 1 {
				t.Errorf("current tabs = %d, want exactly 1", got)
			}
			// Matched on the attribute rather than the class string, so
			// restyling the links does not break the test. The closing quote is
			// load-bearing: the Character href is a prefix of the Spells one.
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
