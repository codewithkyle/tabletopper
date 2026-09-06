package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/prefs"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
)

// settingsForm is a complete, valid submission. A test that is about one field
// changes that field and leaves the rest alone, so nothing passes or fails for
// a reason it did not mean to exercise.
func settingsForm() url.Values {
	return url.Values{
		"theme":       {"dark"},
		"timezone":    {"Europe/London"},
		"date_format": {"iso"},
		"time_format": {"24h"},
	}
}

func saveSettings(t *testing.T, db *recordingDB, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodPost, "/account/settings", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	app.SaveAccountSettings(rec, r)

	return rec
}

// EVERY FIELD IS CHECKED AGAINST THE LIST THAT OFFERED IT, and a value off that
// list stops the whole save. The read path falls back on an unknown value --
// prefs.New has to, so a column this build does not understand still renders --
// and this is the test that the write path does the opposite.
//
// The palette names are in here on purpose. They are what the CSS answers to,
// they are one function call away in the same process, and storing one would
// look like it worked right up until the theme was renamed.
func TestASettingThePickerDoesNotOfferIsRefused(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{name: "a DaisyUI palette instead of the intent", field: "theme", value: "coffee"},
		{name: "the other palette", field: "theme", value: "caramellatte"},
		{name: "an empty theme", field: "theme", value: ""},
		{name: "a real IANA zone the picker does not list", field: "timezone", value: "America/Nipigon"},
		{name: "an offset rather than a zone", field: "timezone", value: "UTC+1"},
		{name: "a path", field: "timezone", value: "../../etc/passwd"},
		{name: "an empty zone", field: "timezone", value: ""},
		{name: "a Go layout instead of a format token", field: "date_format", value: "02/01/2006"},
		{name: "a notation instead of a format token", field: "date_format", value: "DD/MM/YYYY"},
		{name: "an empty date format", field: "date_format", value: ""},
		{name: "a near miss on the clock", field: "time_format", value: "24"},
		{name: "an empty clock", field: "time_format", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &recordingDB{rows: 1}

			form := settingsForm()
			form.Set(tt.field, tt.value)
			rec := saveSettings(t, db, form)

			if len(db.calls) != 0 {
				t.Errorf("statements run = %d, want 0: a rejected form writes nothing", len(db.calls))
			}
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			// 422 is the one 4xx the form has an hx-status route for, and the
			// body is the error block it swaps.
			if !strings.Contains(rec.Body.String(), "errors-account-settings") {
				t.Errorf("the reply is not the error block\n%s", rec.Body.String())
			}
		})
	}
}

// One bad field is a bad form. There is no partial save to explain to anyone,
// and no state where the theme took and the zone did not.
func TestOneBadFieldStopsTheWholeSave(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Set("timezone", "Mars/Olympus_Mons")
	saveSettings(t, db, form)

	if len(db.calls) != 0 {
		t.Fatalf("statements run = %d, want 0", len(db.calls))
	}
}

func TestAValidSaveWritesTheFourColumnsOnce(t *testing.T) {
	db := &recordingDB{rows: 1}

	rec := saveSettings(t, db, settingsForm())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1\n%v", len(db.calls), db.calls)
	}

	call := db.calls[0]
	if want := []string{"theme", "timezone", "date_format", "time_format"}; !equalStrings(setColumns(t, call.query), want) {
		t.Errorf("wrote %v, want %v", setColumns(t, call.query), want)
	}

	// THE ROW IS THE SESSION'S OWN AND NOT ONE NAMED IN THE REQUEST. The route
	// takes no id, so the only user this can reach is the one asking.
	wantArgs := []any{
		queries.UsersTheme("dark"),
		"Europe/London",
		queries.UsersDateFormat("iso"),
		queries.UsersTimeFormat("24h"),
		testOwnerID,
	}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", call.args, wantArgs)
	}
	for i, want := range wantArgs {
		if call.args[i] != want {
			t.Errorf("arg %d = %#v, want %#v", i, call.args[i], want)
		}
	}
}

// THE SAVE HAS TO REPAINT THE PAGE IT IS SITTING ON. The response swapped a
// fragment inside the dialog; the data-theme attribute lives on <html>, which
// nothing in that swap touched, so without this the reader picks Dark, the
// dialog closes, and the page stays light until they navigate.
func TestASaveClosesTheDialogRepaintsAndSaysSo(t *testing.T) {
	db := &recordingDB{rows: 1}

	rec := saveSettings(t, db, settingsForm())

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %q", rec.Header().Get("HX-Trigger"))
	}

	// All three ride one header. Setting it by hand rather than through
	// internal/htmx would have kept whichever was written last.
	for _, name := range []string{"modal:close", "flash:toast", "theme:change"} {
		if _, ok := events[name]; !ok {
			t.Errorf("no %s in %v", name, events)
		}
	}

	// The detail is an object because htmx wraps a bare value as {value: ...},
	// and every other event this app raises reads a named field.
	change, ok := events["theme:change"].(map[string]any)
	if !ok {
		t.Fatalf("theme:change detail = %#v, want an object", events["theme:change"])
	}
	if change["palette"] != "coffee" {
		t.Errorf("palette = %#v, want the dark theme's name", change["palette"])
	}
}

// System is a real answer and its palette is the empty string, which the
// listener reads as "take the attribute off" so the OS media query applies
// again. A save that dropped the event for system would leave the page on
// whichever palette was explicitly set before it.
func TestChoosingSystemRepaintsWithNoPalette(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Set("theme", "system")
	rec := saveSettings(t, db, form)

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %q", rec.Header().Get("HX-Trigger"))
	}

	change, ok := events["theme:change"].(map[string]any)
	if !ok {
		t.Fatalf("theme:change detail = %#v, want an object", events["theme:change"])
	}
	if change["palette"] != "" {
		t.Errorf("palette = %#v, want the empty string", change["palette"])
	}
}

// The dialog opens on what is stored, so every picker has to be handed the
// current value as well as its options.
func TestTheDialogIsBuiltFromTheStoredSettings(t *testing.T) {
	p := prefs.Preferences{
		Theme:      prefs.ThemeDark,
		Timezone:   "Australia/Sydney",
		DateFormat: prefs.DateDMYSlash,
		TimeFormat: prefs.Time24H,
	}

	data := accountSettingsData(p, summerNoon)

	if data.Theme != "dark" || data.Zone != "Australia/Sydney" ||
		data.DateFormat != "dmy_slash" || data.TimeFormat != "24h" {
		t.Errorf("the dialog does not open on what is stored: %#v", data)
	}
	if len(data.Themes) != 3 || len(data.DateFormats) != 5 || len(data.TimeFormats) != 2 {
		t.Errorf("wrong option counts: %d themes, %d date formats, %d clocks",
			len(data.Themes), len(data.DateFormats), len(data.TimeFormats))
	}
	if len(data.Zones) == 0 {
		t.Fatal("no zone groups")
	}
	aliases := 0
	for _, group := range data.Zones {
		if group.Label == "" || len(group.Zones) == 0 {
			t.Errorf("empty zone group %#v", group)
		}
		for _, zone := range group.Zones {
			if zone.Value == "" || zone.Label == "" {
				t.Errorf("incomplete zone option %#v", zone)
			}
			if zone.Alias != "" {
				aliases++
			}
		}
	}
	// The aliases have to survive the trip into the page data, or the welcome
	// dialog's detection silently misses every reader whose browser still
	// reports the pre-rename spelling.
	if aliases == 0 {
		t.Error("no zone carries its older IANA spelling into the markup")
	}
}

// THE OPTIONS SHOW THE ANSWER RATHER THAN THE NOTATION, because "DD/MM/YYYY" is
// something the reader has to decode and "06/09/2026" is something they can
// recognise. The two numeric ones keep the notation as well, since on the days
// when the day and month are the same number they render identically.
//
// The examples are rendered in the SAVED zone, which is why the Sydney reader
// below is looking at the 7th while it is still the 6th in UTC.
func TestTheOptionsAreLabelledWithRealDates(t *testing.T) {
	p := prefs.Preferences{
		Theme:      prefs.ThemeSystem,
		Timezone:   "Australia/Sydney",
		DateFormat: prefs.DateDMYText,
		TimeFormat: prefs.Time12H,
	}

	data := accountSettingsData(p, summerNoon)

	want := map[string]string{
		"dmy_text":  "7 Sep 2026",
		"mdy_text":  "Sep 7, 2026",
		"mdy_slash": "09/07/2026 (MM/DD/YYYY)",
		"dmy_slash": "07/09/2026 (DD/MM/YYYY)",
		"iso":       "2026-09-07",
	}
	for _, option := range data.DateFormats {
		if got := want[option.Value]; option.Label != got {
			t.Errorf("date option %q labelled %q, want %q", option.Value, option.Label, got)
		}
	}

	wantClocks := map[string]string{
		"12h": "12-hour (4:04 AM)",
		"24h": "24-hour (04:04)",
	}
	for _, option := range data.TimeFormats {
		if got := wantClocks[option.Value]; option.Label != got {
			t.Errorf("clock option %q labelled %q, want %q", option.Value, option.Label, got)
		}
	}

	// The theme options name the behaviour, not the palette. A reader has met
	// neither "caramellatte" nor "coffee".
	for _, option := range data.Themes {
		if strings.Contains(strings.ToLower(option.Label), "caramellatte") ||
			strings.Contains(strings.ToLower(option.Label), "coffee") {
			t.Errorf("theme option %q leaks a palette name: %q", option.Value, option.Label)
		}
	}
}

// summerNoon is 18:04 UTC on 6 September 2026 -- the same instant the prefs
// tests use, chosen because the day and the month are different numbers, so the
// two slash formats render differently and a swap between them would show.
var summerNoon = time.Date(2026, 9, 6, 18, 4, 11, 0, time.UTC)

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func welcomePost(t *testing.T, db *recordingDB, path string, form url.Values, handler func(*App, http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
	t.Helper()

	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(app, rec, r)

	return rec
}

// THE SETTINGS AND THE STAMP GO IN ONE STATEMENT. Two would leave a window in
// which the answer landed and the account was still marked unset up, and the
// next page load would reopen the welcome dialog over the choice just made.
func TestFinishingTheWelcomeWritesTheSettingsAndTheStampTogether(t *testing.T) {
	db := &recordingDB{rows: 1}

	rec := welcomePost(t, db, "/account/welcome", settingsForm(),
		func(a *App, w http.ResponseWriter, r *http.Request) { a.CompleteOnboarding(w, r) })

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}

	want := []string{"theme", "timezone", "date_format", "time_format", "onboarded_at"}
	if got := setColumns(t, db.calls[0].query); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
	// COALESCE, so a second answer keeps the first stamp rather than moving it.
	if !strings.Contains(db.calls[0].query, "COALESCE(onboarded_at") {
		t.Errorf("the stamp is not preserved across a re-answer:\n%s", db.calls[0].query)
	}
}

// The welcome dialog validates exactly as the settings dialog does, because it
// is the same four pickers. A rejected form writes nothing -- and crucially
// leaves the account unstamped, so the question is asked again rather than
// being silently closed on a value that never landed.
func TestARejectedWelcomeLeavesTheAccountUnstamped(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Set("date_format", "DD/MM/YYYY")
	rec := welcomePost(t, db, "/account/welcome", form,
		func(a *App, w http.ResponseWriter, r *http.Request) { a.CompleteOnboarding(w, r) })

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Errorf("statements run = %d, want 0", len(db.calls))
	}
	if !strings.Contains(rec.Body.String(), "errors-account-welcome") {
		t.Errorf("the reply is not the welcome dialog's error block\n%s", rec.Body.String())
	}
}

// "NOT NOW" READS NO FORM. The button sits inside the welcome form and htmx may
// send its values along; storing them would turn pickers the reader explicitly
// declined -- including a zone the browser guessed for them -- into their saved
// answer. The statement takes one argument, and it is the account's own id.
func TestNotNowStampsTheAccountAndStoresNothingElse(t *testing.T) {
	db := &recordingDB{rows: 1}

	// A form that would be perfectly valid, to prove it is ignored rather than
	// merely absent.
	form := settingsForm()
	form.Set("theme", "light")
	rec := welcomePost(t, db, "/account/welcome/skip", form,
		func(a *App, w http.ResponseWriter, r *http.Request) { a.DismissOnboarding(w, r) })

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}

	call := db.calls[0]
	if got := setColumns(t, call.query); !equalStrings(got, []string{"onboarded_at"}) {
		t.Errorf("wrote %v, want only the stamp", got)
	}
	if len(call.args) != 1 || call.args[0] != testOwnerID {
		t.Errorf("args = %v, want just the session's own id", call.args)
	}

	// It closes and hands over: this dialog is not coming back, so the toast is
	// the only chance to say where the settings went.
	trigger := rec.Header().Get("HX-Trigger")
	for _, want := range []string{"modal:close", "flash:toast"} {
		if !strings.Contains(trigger, want) {
			t.Errorf("HX-Trigger %q missing %s", trigger, want)
		}
	}
	if !strings.Contains(trigger, "gear") {
		t.Errorf("the toast does not say where to find the settings: %q", trigger)
	}
	// Nothing changed, so nothing is repainted.
	if strings.Contains(trigger, "theme:change") {
		t.Errorf("a dismissal repainted the page: %q", trigger)
	}
}
