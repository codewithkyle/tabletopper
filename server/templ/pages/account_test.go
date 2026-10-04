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
		PingVolume: prefs.VolumeMax,
		TurnAlert:  true,
		TurnVolume: prefs.VolumeMax,
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
		TimeFormat:  "24h",
		FollowTurn:  true,
		ShowBlood:   true,
		MusicVolume: prefs.MusicVolumeDefault,
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
		`name="turn_volume"`,
		`name="music_volume"`,
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
		{name: "the on-deck alert", field: "turn_alert", off: func(d *AccountSettingsData) { d.TurnAlert = false }},
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
		`max="` + VolumeMax + `"`,
		`step="` + VolumeStep + `"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the Sounds section is missing %s\n%s", want, markup)
		}
	}
}
func TestThePingVolumeSliderOpensOnWhatIsStored(t *testing.T) {
	full := collapseWhitespace(renderSettings(t))
	if !strings.Contains(full, `value="`+VolumeMax+`"`) {
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
	for v := 0; v <= prefs.VolumeMax; v += prefs.VolumeStep {
		if got, ok := prefs.ParseVolume(strconv.Itoa(v), prefs.Default.PingVolume); !ok || got != v {
			t.Errorf("the save refuses %d, which the slider offers (got %d, ok %v)", v, got, ok)
		}
	}
}
func TestTheWelcomeDialogCarriesBothTableToggles(t *testing.T) {
	welcome := collapseWhitespace(renderWelcome(t))
	for _, field := range []string{"follow_turn", "show_blood", "ping_volume", "turn_alert", "turn_volume", "turn_notify", "music_volume"} {
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

func TestTheOnDeckAlertHasBothItsToggleAndItsVolume(t *testing.T) {
	markup := collapseWhitespace(renderSettings(t))
	for _, want := range []string{
		`name="turn_alert" type="checkbox"`,
		`name="turn_notify" type="checkbox"`,
		`name="turn_volume" type="range" min="0"`,
		`data-range-output="` + TurnVolumeOutputID + `"`,
		`id="` + TurnVolumeOutputID + `"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("the settings dialog is missing %s\n%s", want, markup)
		}
	}
}
func TestEveryVolumeSliderReadsIntoItsOwnPlace(t *testing.T) {
	ids := map[string]string{
		"ping":  PingVolumeOutputID,
		"turn":  TurnVolumeOutputID,
		"music": MusicVolumeOutputID,
	}
	seen := map[string]string{}
	for name, id := range ids {
		if other, taken := seen[id]; taken {
			t.Fatalf("%s and %s write into one reading", name, other)
		}
		seen[id] = name
	}
	data := testAccountSettings()
	data.PingVolume = 30
	data.TurnVolume = 70
	data.MusicVolume = 45
	rendered := collapseWhitespace(markup(t, AccountSettingsFragment(data)))
	for _, want := range []int{30, 70, 45} {
		if !strings.Contains(rendered, `<span data-range-value>`+strconv.Itoa(want)+`</span>%`) {
			t.Errorf("no reading opened on %d\n%s", want, rendered)
		}
	}
}
func TestTheMusicSliderOpensOnAQuarterForANewAccount(t *testing.T) {
	if prefs.Default.MusicVolume != 25 {
		t.Errorf("a new account's music volume = %d, want 25", prefs.Default.MusicVolume)
	}
	if prefs.MusicVolumeDefault%prefs.VolumeStep != 0 {
		t.Errorf("the default %d is not a position the slider can land on, which is every %d", prefs.MusicVolumeDefault, prefs.VolumeStep)
	}
	if _, ok := prefs.ParseVolume(strconv.Itoa(prefs.MusicVolumeDefault), 0); !ok {
		t.Errorf("the save refuses the default the slider opens on")
	}
}
func TestOnlyTheSynthesisedSoundsPreviewThemselves(t *testing.T) {
	rendered := collapseWhitespace(renderSettings(t))
	for _, kind := range []string{"ping", "turn"} {
		if !strings.Contains(rendered, `data-sound-preview="`+kind+`"`) {
			t.Errorf("the %s slider previews nothing when it is let go\n%s", kind, rendered)
		}
	}
	if got := strings.Count(rendered, "data-sound-preview="); got != 2 {
		t.Errorf("%d sliders preview themselves, want the two the browser synthesises\n%s", got, rendered)
	}
	musicAt := strings.Index(rendered, `name="music_volume"`)
	if musicAt < 0 {
		t.Fatalf("no music slider\n%s", rendered)
	}
	tag := rendered[musicAt:]
	if end := strings.Index(tag, ">"); end >= 0 {
		tag = tag[:end]
	}
	if strings.Contains(tag, "data-sound-preview") {
		t.Errorf("the music slider previews itself, but there is no track to play\n%s", tag)
	}
	if strings.Contains(rendered, ">Test<") {
		t.Errorf("a test button survived\n%s", rendered)
	}
}
func TestOnlyTheSoundsThatNeedExplainingCarryADescription(t *testing.T) {
	rendered := collapseWhitespace(renderSettings(t))
	if strings.Contains(rendered, `for="music-volume"</label><p`) {
		t.Errorf("the music slider carries a description it does not need\n%s", rendered)
	}
	if got := strings.Count(rendered, `<p class="m-0 text-sm text-base-content/75">How loud`); got != 2 {
		t.Errorf("%d volume sliders explain themselves, want the two that are not obvious\n%s", got, rendered)
	}
}
func TestTheSettingsSplitIntoTwoColumnsWithTheWideRowsSpanningThem(t *testing.T) {
	rendered := collapseWhitespace(renderSettings(t))
	if got := strings.Count(rendered, "sm:grid-cols-2"); got != 2 {
		t.Errorf("%d two-column grids, want the fields and the sounds\n%s", got, rendered)
	}
	if !strings.Contains(rendered, "sm:col-span-2") {
		t.Errorf("the third slider does not span the two columns beside it\n%s", rendered)
	}
	data := testAccountSettings()
	data.Storage = "1.5 GB"
	full := markup(t, AccountSettingsFragment(data))
	soundsAt := strings.Index(full, "music_volume")
	storageAt := strings.Index(full, "Storage used")
	actionsAt := strings.Index(full, "modal-action")
	switch {
	case storageAt < 0:
		t.Fatalf("no usage row\n%s", full)
	case storageAt < soundsAt:
		t.Errorf("the usage row sits inside the columns rather than under them\n%s", full)
	case storageAt > actionsAt:
		t.Errorf("the usage row sits below the buttons\n%s", full)
	}
}
func TestDeleteAccountSitsInTheActionRowLeftOfClose(t *testing.T) {
	rendered := renderSettings(t)
	actionsAt := strings.Index(rendered, "modal-action")
	deleteAt := strings.Index(rendered, ">Delete account<")
	closeAt := strings.Index(rendered, ">Close<")
	saveAt := strings.Index(rendered, ">Save settings<")
	switch {
	case deleteAt < 0:
		t.Fatalf("no way to delete the account\n%s", rendered)
	case deleteAt < actionsAt:
		t.Errorf("Delete account is above the action row\n%s", rendered)
	case deleteAt > closeAt:
		t.Errorf("Delete account is not to the left of Close\n%s", rendered)
	case closeAt > saveAt:
		t.Errorf("Close comes after the affirmative action\n%s", rendered)
	}
	if !strings.Contains(rendered, `hx-params="none"`) {
		t.Errorf("the delete posts the settings form it now sits inside\n%s", rendered)
	}
	if !strings.Contains(rendered, `data-confirm-label="Delete my account"`) {
		t.Errorf("the delete is not gated by a confirm\n%s", rendered)
	}
	if strings.Contains(renderWelcome(t), ">Delete account<") {
		t.Error("the welcome dialog offers to delete the account it just made")
	}
}
func TestTheNotificationToggleOwnsTheHintAboutBeingRefused(t *testing.T) {
	markup := collapseWhitespace(renderSettings(t))
	if !strings.Contains(markup, "data-notify-permission") {
		t.Errorf("nothing asks the browser for permission\n%s", markup)
	}
	if !strings.Contains(markup, "data-notify-blocked hidden") {
		t.Errorf("the refused hint is missing, or is on screen before it is true\n%s", markup)
	}
}
