package pages

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"

	"tabletopper/internal/prefs"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

func testAccountSettings() AccountSettingsData {
	return AccountSettingsData{
		Themes: []Option{
			{Value: "system", Label: "Follow system"},
			{Value: "light", Label: "Light"},
			{Value: "dark", Label: "Dark"},
		},
		Theme:      "dark",
		PingVolume: prefs.PingVolumeMax,
		Zones: []ZoneGroup{
			{Label: "Universal", Zones: []ZoneOption{{Value: "UTC", Label: "UTC"}}},
			{Label: "Americas", Zones: []ZoneOption{
				{Value: "America/New_York", Label: "New York"},
				{Value: "America/Chicago", Label: "Chicago"},
				{Value: "America/Argentina/Buenos_Aires", Label: "Buenos Aires", Alias: "America/Buenos_Aires"},
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
		FollowTurn: true,
		ShowBlood:  true,
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
	if got := strings.Count(markup, " selected>"); got != 4 {
		t.Errorf("%d options are selected, want 4\n%s", got, markup)
	}
}
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
	if got := strings.Count(markup, "<optgroup"); got != 2 {
		t.Errorf("%d optgroups, want 2 -- only the zone picker is grouped\n%s", got, markup)
	}
}
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
func TestTheGearIsOnlyThereForSomeoneSignedIn(t *testing.T) {
	signedOut := renderHomepage(t, session.UserSession{})
	if strings.Contains(signedOut, "/fragment/account/settings") {
		t.Errorf("the settings gear is on the signed-out homepage\n%s", signedOut)
	}
	signedIn := renderHomepage(t, session.UserSession{UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ")})
	if !strings.Contains(signedIn, `data-modal-open="/fragment/account/settings"`) {
		t.Errorf("no settings gear for a signed-in reader\n%s", signedIn)
	}
	if !strings.Contains(signedIn, `data-modal-open="/fragment/`) {
		t.Errorf("the gear does not name a /fragment/ route\n%s", signedIn)
	}
}
func TestTheShellPaintsTheReadersTheme(t *testing.T) {
	tests := []struct {
		name  string
		theme prefs.Theme
		want  string
	}{
		{name: "light takes the light palette", theme: prefs.ThemeLight, want: `<html data-theme="caramellatte">`},
		{name: "dark takes the dark one", theme: prefs.ThemeDark, want: `<html data-theme="coffee">`},
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
func TestASignedOutPageIsNotThemed(t *testing.T) {
	markup := renderHomepage(t, session.UserSession{})
	if !strings.Contains(markup, "<html>") {
		t.Errorf("the signed-out homepage carries a theme\n%s", markup[:min(len(markup), 400)])
	}
}
func TestTheProfileWidgetCarriesTheUploadBadge(t *testing.T) {
	markup := renderHomepage(t, session.UserSession{
		UserID:          ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ"),
		Username:        "kyle",
		ProfileImageURL: "/images/default-avatar.webp",
	})
	for _, want := range []string{
		`hx-post="/account/avatar"`,
		`hx-target="#account-avatar"`,
		`hx-encoding="multipart/form-data"`,
		`name="avatar"`,
		`accept="image/png, image/jpeg, image/webp"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the profile widget is missing %s", want)
		}
	}
	if strings.Contains(markup, `hx-target="#user-badge"`) {
		t.Error("the upload swaps the whole badge, which replays the page's fade-in")
	}
}
func TestTheProfileWidgetDrawsTheResolvedPicture(t *testing.T) {
	uploaded := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVWX")
	markup := renderHomepage(t, session.UserSession{
		UserID:          ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ"),
		Username:        "kyle",
		ProfileImageURL: session.AvatarURL(&uploaded, "https://img.clerk.com/kyle"),
	})
	if !strings.Contains(markup, `src="/assets/images/`+uploaded.String()+`"`) {
		t.Error("the widget does not draw the uploaded picture")
	}
	if strings.Contains(markup, "img.clerk.com") {
		t.Error("the widget still draws the Clerk picture the upload overrides")
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
func renderWelcome(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := AccountWelcomeFragment(testAccountSettings()).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}
func TestTheWelcomeAndSettingsDialogsOfferTheSameFields(t *testing.T) {
	settings := collapseWhitespace(renderSettings(t))
	welcome := collapseWhitespace(renderWelcome(t))
	for _, want := range []string{
		`name="theme"`,
		`name="timezone"`,
		`name="date_format"`,
		`name="time_format"`,
		`name="follow_turn"`,
		`name="show_blood"`,
		`name="ping_volume"`,
		`<optgroup label="Americas">`,
		`<option value="dark" selected>Dark</option>`,
		`<option value="America/Chicago" selected>Chicago</option>`,
		`<option value="iso" selected>2026-09-06</option>`,
		`<option value="24h" selected>24-hour (14:04)</option>`,
	} {
		if !strings.Contains(settings, want) {
			t.Errorf("the settings dialog is missing %s", want)
		}
		if !strings.Contains(welcome, want) {
			t.Errorf("the welcome dialog is missing %s", want)
		}
	}
}
func TestTheTableTogglesOpenOnWhatIsStored(t *testing.T) {
	tests := []struct {
		name  string
		field string
		off   func(*AccountSettingsData)
	}{
		{name: "the camera", field: "follow_turn", off: func(d *AccountSettingsData) { d.FollowTurn = false }},
		{name: "the blood", field: "show_blood", off: func(d *AccountSettingsData) { d.ShowBlood = false }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ticked := `name="` + tc.field + `" type="checkbox" class="toggle col-start-2 row-start-1" checked`
			on := collapseWhitespace(renderSettings(t))
			if !strings.Contains(on, ticked) {
				t.Errorf("a stored true did not render ticked\n%s", on)
			}
			data := testAccountSettings()
			tc.off(&data)
			var buf bytes.Buffer
			if err := AccountSettingsFragment(data).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			off := collapseWhitespace(buf.String())
			if strings.Contains(off, ticked) {
				t.Errorf("a stored false rendered ticked\n%s", off)
			}
			if !strings.Contains(off, `name="`+tc.field+`"`) {
				t.Errorf("the toggle is missing altogether\n%s", off)
			}
			for _, other := range tests {
				if other.field == tc.field {
					continue
				}
				if !strings.Contains(off, `name="`+other.field+`" type="checkbox" class="toggle col-start-2 row-start-1" checked`) {
					t.Errorf("turning off %s also unticked %s\n%s", tc.field, other.field, off)
				}
			}
		})
	}
}
func TestThePingVolumeSliderRunsFromSilentToFull(t *testing.T) {
	markup := collapseWhitespace(renderSettings(t))
	for _, want := range []string{
		`<span class="fieldset-legend">Sounds</span>`,
		`name="ping_volume" type="range" min="0"`,
		`max="` + PingVolumeMax + `"`,
		`step="` + PingVolumeStep + `"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the Sounds section is missing %s\n%s", want, markup)
		}
	}
}
func TestThePingVolumeSliderOpensOnWhatIsStored(t *testing.T) {
	full := collapseWhitespace(renderSettings(t))
	if !strings.Contains(full, `value="`+PingVolumeMax+`"`) {
		t.Errorf("a stored full volume did not reach the slider\n%s", full)
	}
	data := testAccountSettings()
	data.PingVolume = 30
	quiet := collapseWhitespace(markup(t, AccountSettingsFragment(data)))
	if !strings.Contains(quiet, `value="30"`) {
		t.Errorf("a stored 30 did not reach the slider\n%s", quiet)
	}
}
func TestThePingVolumeSliderWritesIntoItsOwnReading(t *testing.T) {
	data := testAccountSettings()
	data.PingVolume = 30
	rendered := collapseWhitespace(markup(t, AccountSettingsFragment(data)))
	if !strings.Contains(rendered, `data-range-output="`+PingVolumeOutputID+`"`) {
		t.Errorf("the slider names no reading\n%s", rendered)
	}
	if !strings.Contains(rendered, `id="`+PingVolumeOutputID+`"`) {
		t.Errorf("the reading the slider names is not on the page\n%s", rendered)
	}
	if !strings.Contains(rendered, `<span data-range-value>30</span>%`) {
		t.Errorf("the reading did not open on the stored value\n%s", rendered)
	}
}
func TestEveryPositionOnTheSliderCanBeSaved(t *testing.T) {
	for v := 0; v <= prefs.PingVolumeMax; v += prefs.PingVolumeStep {
		if got, ok := prefs.ParsePingVolume(strconv.Itoa(v)); !ok || got != v {
			t.Errorf("the save refuses %d, which the slider offers (got %d, ok %v)", v, got, ok)
		}
	}
}
func TestTheWelcomeDialogCarriesBothTableToggles(t *testing.T) {
	welcome := collapseWhitespace(renderWelcome(t))
	for _, field := range []string{"follow_turn", "show_blood", "ping_volume"} {
		if !strings.Contains(welcome, `name="`+field+`"`) {
			t.Errorf("the welcome dialog would post no answer for %s, which its save writes", field)
		}
	}
}
func TestAZonesOlderNameIsRenderedBesideIt(t *testing.T) {
	markup := collapseWhitespace(renderWelcome(t))
	if !strings.Contains(markup, `data-alias="America/Buenos_Aires"`) {
		t.Errorf("the alias did not reach the markup\n%s", markup)
	}
	if strings.Contains(markup, `value="America/Buenos_Aires"`) {
		t.Errorf("an alias is offered as a storable value\n%s", markup)
	}
	if got := strings.Count(markup, "data-alias"); got != 1 {
		t.Errorf("%d options carry an alias, want only the one that has one\n%s", got, markup)
	}
}
func TestOnlyTheWelcomeDialogGuessesTheZone(t *testing.T) {
	if !strings.Contains(renderWelcome(t), "<zone-detect") {
		t.Errorf("the welcome dialog does not offer a detected zone\n%s", renderWelcome(t))
	}
	if strings.Contains(renderSettings(t), "zone-detect") {
		t.Errorf("the settings dialog would overwrite a stored zone with a guess\n%s", renderSettings(t))
	}
}
func TestTheWelcomeDialogsTwoActions(t *testing.T) {
	markup := renderWelcome(t)
	notNow := strings.Index(markup, ">Not now<")
	save := strings.Index(markup, ">Save and get started<")
	switch {
	case notNow < 0:
		t.Fatalf("no way to decline\n%s", markup)
	case save < 0:
		t.Fatalf("no affirmative action\n%s", markup)
	case notNow > save:
		t.Errorf("the close control comes after the affirmative action\n%s", markup)
	}
	if !strings.Contains(markup, `hx-post="/account/welcome"`) {
		t.Errorf("the form does not post to /account/welcome\n%s", markup)
	}
	if !strings.Contains(markup, `hx-post="/account/welcome/skip"`) {
		t.Errorf("Not now does not post the dismissal\n%s", markup)
	}
	if !strings.Contains(markup, `type="button"`) {
		t.Errorf("Not now submits the form\n%s", markup)
	}
	if !strings.Contains(markup, `id="errors-account-welcome"`) {
		t.Errorf("no error block of its own\n%s", markup)
	}
}
func TestTheWelcomeOpensItselfOnlyForAnAccountThatHasNotAnswered(t *testing.T) {
	newAccount := renderHomepage(t, session.UserSession{
		UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ"),
	})
	if !strings.Contains(newAccount, `data-modal-autoopen="/fragment/account/welcome"`) {
		t.Errorf("a new account is not asked\n%s", newAccount)
	}
	answered := renderHomepage(t, session.UserSession{
		UserID:    ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ"),
		Onboarded: true,
	})
	if strings.Contains(answered, "data-modal-autoopen") {
		t.Errorf("an account that has answered is asked again\n%s", answered)
	}
	if strings.Contains(renderHomepage(t, session.UserSession{}), "data-modal-autoopen") {
		t.Errorf("the signed-out homepage opens a welcome dialog")
	}
}
