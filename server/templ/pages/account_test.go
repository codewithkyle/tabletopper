package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tabletopper/internal/prefs"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// testAccountSettings is a dialog with something selected in every picker, so a
// test can tell "selected the stored value" from "selected the first option".
func testAccountSettings() AccountSettingsData {
	return AccountSettingsData{
		Themes: []Option{
			{Value: "system", Label: "Follow system"},
			{Value: "light", Label: "Light"},
			{Value: "dark", Label: "Dark"},
		},
		Theme: "dark",
		Zones: []OptionGroup{
			{Label: "Universal", Options: []Option{{Value: "UTC", Label: "UTC"}}},
			{Label: "Americas", Options: []Option{
				{Value: "America/New_York", Label: "New York"},
				{Value: "America/Chicago", Label: "Chicago"},
			}},
		},
		Zone: "America/Chicago",
		DateFormats: []Option{
			{Value: "dmy_text", Label: "6 Sep 2026"},
			{Value: "iso", Label: "2026-09-06"},
		},
		DateFormat: "iso",
		TimeFormats: []Option{
			{Value: "12h", Label: "12-hour (2:04 PM)"},
			{Value: "24h", Label: "24-hour (14:04)"},
		},
		TimeFormat: "24h",
	}
}

func renderSettings(t *testing.T) string {
	t.Helper()

	var buf bytes.Buffer
	if err := AccountSettingsFragment(testAccountSettings()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	return buf.String()
}

// Every picker opens on what is stored. A dialog that showed the first option
// instead would read as "your theme is Follow system" to somebody who had
// chosen Dark, and saving without touching it would quietly undo their choice.
func TestTheSettingsDialogOpensOnWhatIsStored(t *testing.T) {
	markup := collapseWhitespace(renderSettings(t))

	for _, want := range []string{
		`<option value="dark" selected>Dark</option>`,
		`<option value="America/Chicago" selected>Chicago</option>`,
		`<option value="iso" selected>2026-09-06</option>`,
		`<option value="24h" selected>24-hour (14:04)</option>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}

	// And exactly one option per select is selected, or the browser takes the
	// last one and the dialog lies about three of the four.
	if got := strings.Count(markup, " selected>"); got != 4 {
		t.Errorf("%d options are selected, want 4\n%s", got, markup)
	}
}

// The zone picker is the only one long enough to need grouping, and the groups
// are what make fifty entries scannable.
func TestTheZonePickerIsGrouped(t *testing.T) {
	markup := collapseWhitespace(renderSettings(t))

	for _, want := range []string{
		`<optgroup label="Universal">`,
		`<optgroup label="Americas">`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("missing %s\n%s", want, markup)
		}
	}

	// The other three are flat. A group of three is a rule with no reader.
	if got := strings.Count(markup, "<optgroup"); got != 2 {
		t.Errorf("%d optgroups, want 2 -- only the zone picker is grouped\n%s", got, markup)
	}
}

// The dialog posts to its own resource URL, not to /fragment/, and routes a 422
// at its error block. 422 is the one 4xx base.templ's noSwap config lets
// through; any other status would leave the dialog showing nothing new.
func TestTheSettingsDialogPostsToItsResourceURL(t *testing.T) {
	markup := renderSettings(t)

	if !strings.Contains(markup, `hx-post="/account/settings"`) {
		t.Errorf("the form does not post to /account/settings\n%s", markup)
	}
	if strings.Contains(markup, `hx-post="/fragment`) {
		t.Errorf("a mutation is posting under /fragment/\n%s", markup)
	}
	if !strings.Contains(markup, `hx-status:422="target:#errors-account-settings,swap:outerHTML"`) {
		t.Errorf("no 422 route to the error block\n%s", markup)
	}
	if !strings.Contains(markup, `id="errors-account-settings"`) {
		t.Errorf("no error block to route one to\n%s", markup)
	}
}

// Close comes first and is a labelled button, like every other dialog in the
// app. It dispatches modal:close rather than being a method="dialog" form,
// because forms do not nest and it sits inside the settings form.
func TestTheSettingsDialogClosesTheWayEveryOtherOneDoes(t *testing.T) {
	markup := renderSettings(t)

	closeAt := strings.Index(markup, ">Close<")
	saveAt := strings.Index(markup, ">Save settings<")
	switch {
	case closeAt < 0:
		t.Fatalf("no Close button\n%s", markup)
	case saveAt < 0:
		t.Fatalf("no affirmative action\n%s", markup)
	case closeAt > saveAt:
		t.Errorf("Close comes after the affirmative action\n%s", markup)
	}

	if !strings.Contains(markup, "modal:close") {
		t.Errorf("Close does not dispatch modal:close\n%s", markup)
	}
	if strings.Contains(markup, `method="dialog"`) {
		t.Errorf("a nested form is trying to close the dialog\n%s", markup)
	}
	if strings.Contains(markup, "<dialog") {
		t.Errorf("the fragment brings a dialog of its own\n%s", markup)
	}
}

// THE GEAR IS FOR AN ACCOUNT, SO IT NEEDS ONE. The icon row renders on both
// halves of the homepage, and the signed-out half has no settings to open.
func TestTheGearIsOnlyThereForSomeoneSignedIn(t *testing.T) {
	signedOut := renderHomepage(t, session.UserSession{})
	if strings.Contains(signedOut, "/fragment/account/settings") {
		t.Errorf("the settings gear is on the signed-out homepage\n%s", signedOut)
	}

	signedIn := renderHomepage(t, session.UserSession{UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ")})
	if !strings.Contains(signedIn, `data-modal-open="/fragment/account/settings"`) {
		t.Errorf("no settings gear for a signed-in reader\n%s", signedIn)
	}
	// The content modal refuses any URL that is not a fragment, so a gear
	// naming a page URL would open onto a spinner that never resolves.
	if !strings.Contains(signedIn, `data-modal-open="/fragment/`) {
		t.Errorf("the gear does not name a /fragment/ route\n%s", signedIn)
	}
}

// THE THEME IS AN ATTRIBUTE ON <html> WRITTEN BY THE SERVER, which is what
// makes it survive a reload with no flash of the wrong palette. It is read off
// the request context rather than passed to Base, because thirteen pages call
// that shell and none of them has an opinion about the theme.
func TestTheShellPaintsTheReadersTheme(t *testing.T) {
	tests := []struct {
		name  string
		theme prefs.Theme
		want  string
	}{
		{name: "light takes the light palette", theme: prefs.ThemeLight, want: `<html data-theme="caramellatte">`},
		{name: "dark takes the dark one", theme: prefs.ThemeDark, want: `<html data-theme="coffee">`},
		// SYSTEM IS AN ABSENT ATTRIBUTE AND NOT A THIRD VALUE. DaisyUI puts the
		// dark theme behind `:root:not([data-theme])` inside a
		// prefers-color-scheme query, so writing anything here would pin the
		// page and stop it following the OS.
		{name: "system writes nothing at all", theme: prefs.ThemeSystem, want: `<html>`},
		{name: "so does the zero value", theme: "", want: `<html>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markup := renderHomepageWithTheme(t, tt.theme)

			if !strings.Contains(markup, tt.want) {
				t.Errorf("missing %s\n%s", tt.want, markup[:min(len(markup), 400)])
			}
		})
	}
}

// A stranger gets the OS preference, because the app knows nothing about them.
func TestASignedOutPageIsNotThemed(t *testing.T) {
	markup := renderHomepage(t, session.UserSession{})

	if !strings.Contains(markup, "<html>") {
		t.Errorf("the signed-out homepage carries a theme\n%s", markup[:min(len(markup), 400)])
	}
}

func renderHomepage(t *testing.T, sess session.UserSession) string {
	t.Helper()

	var buf bytes.Buffer
	ctx := session.NewContext(context.Background(), sess)
	if err := Homepage(sess).Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	return buf.String()
}

func renderHomepageWithTheme(t *testing.T, theme prefs.Theme) string {
	t.Helper()

	return renderHomepage(t, session.UserSession{
		UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ"),
		Prefs:  prefs.Preferences{Theme: theme},
	})
}
