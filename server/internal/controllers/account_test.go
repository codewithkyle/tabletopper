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




func settingsForm() url.Values {
	return url.Values{
		"username":    {"kyle"},
		"theme":       {"dark"},
		"timezone":    {"Europe/London"},
		"date_format": {"iso"},
		"time_format": {"24h"},
		"follow_turn": {"on"},
		"show_blood":  {"on"},
		"ping_volume": {"40"},
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
			
			
			if !strings.Contains(rec.Body.String(), "errors-account-settings") {
				t.Errorf("the reply is not the error block\n%s", rec.Body.String())
			}
		})
	}
}



func TestOneBadFieldStopsTheWholeSave(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Set("timezone", "Mars/Olympus_Mons")
	saveSettings(t, db, form)

	if len(db.calls) != 0 {
		t.Fatalf("statements run = %d, want 0", len(db.calls))
	}
}

func TestAValidSaveWritesTheEightColumnsOnce(t *testing.T) {
	db := &recordingDB{rows: 1}

	rec := saveSettings(t, db, settingsForm())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1\n%v", len(db.calls), db.calls)
	}

	call := db.calls[0]
	if want := []string{"username", "theme", "timezone", "date_format", "time_format", "follow_turn", "show_blood", "ping_volume"}; !equalStrings(setColumns(t, call.query), want) {
		t.Errorf("wrote %v, want %v", setColumns(t, call.query), want)
	}

	
	
	wantArgs := []any{
		"kyle",
		queries.UsersTheme("dark"),
		"Europe/London",
		queries.UsersDateFormat("iso"),
		queries.UsersTimeFormat("24h"),
		true,
		true,
		uint8(40),
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





func TestASaveClosesTheDialogRepaintsAndSaysSo(t *testing.T) {
	db := &recordingDB{rows: 1}

	rec := saveSettings(t, db, settingsForm())

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %q", rec.Header().Get("HX-Trigger"))
	}

	
	
	for _, name := range []string{"modal:close", "flash:toast", "theme:change", "settings:change"} {
		if _, ok := events[name]; !ok {
			t.Errorf("no %s in %v", name, events)
		}
	}

	
	
	change, ok := events["theme:change"].(map[string]any)
	if !ok {
		t.Fatalf("theme:change detail = %#v, want an object", events["theme:change"])
	}
	if change["palette"] != "coffee" {
		t.Errorf("palette = %#v, want the dark theme's name", change["palette"])
	}
}





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



func TestTheDialogIsBuiltFromTheStoredSettings(t *testing.T) {
	p := prefs.Preferences{
		Theme:      prefs.ThemeDark,
		Timezone:   "Australia/Sydney",
		DateFormat: prefs.DateDMYSlash,
		TimeFormat: prefs.Time24H,
	}

	data := accountSettingsData("kyle", p, summerNoon)

	if data.Name != "kyle" {
		t.Errorf("the dialog opens on display name %q, want %q", data.Name, "kyle")
	}
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
	
	
	
	if aliases == 0 {
		t.Error("no zone carries its older IANA spelling into the markup")
	}
}








func TestTheOptionsAreLabelledWithRealDates(t *testing.T) {
	p := prefs.Preferences{
		Theme:      prefs.ThemeSystem,
		Timezone:   "Australia/Sydney",
		DateFormat: prefs.DateDMYText,
		TimeFormat: prefs.Time12H,
	}

	data := accountSettingsData("kyle", p, summerNoon)

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

	
	
	for _, option := range data.Themes {
		if strings.Contains(strings.ToLower(option.Label), "caramellatte") ||
			strings.Contains(strings.ToLower(option.Label), "coffee") {
			t.Errorf("theme option %q leaks a palette name: %q", option.Value, option.Label)
		}
	}
}




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

	want := []string{"username", "theme", "timezone", "date_format", "time_format", "follow_turn", "show_blood", "ping_volume", "onboarded_at"}
	if got := setColumns(t, db.calls[0].query); !equalStrings(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
	
	if !strings.Contains(db.calls[0].query, "COALESCE(onboarded_at") {
		t.Errorf("the stamp is not preserved across a re-answer:\n%s", db.calls[0].query)
	}
}





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








func TestUntickingATogglePutsItAway(t *testing.T) {
	tests := []struct {
		name  string
		field string
		arg   int
		other int
	}{
		
		
		
		{name: "the camera", field: "follow_turn", arg: 5, other: 6},
		{name: "the blood", field: "show_blood", arg: 6, other: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := &recordingDB{rows: 1}

			form := settingsForm()
			form.Del(tc.field)
			rec := saveSettings(t, db, form)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			if len(db.calls) != 1 {
				t.Fatalf("statements run = %d, want 1", len(db.calls))
			}

			if got := db.calls[0].args[tc.arg]; got != false {
				t.Errorf("%s = %#v, want false", tc.field, got)
			}

			
			
			
			if got := db.calls[0].args[tc.other]; got != true {
				t.Errorf("arg %d = %#v, want the other toggle left on", tc.other, got)
			}
		})
	}
}





func TestTheWelcomeDialogSavesTheTableSettingsToo(t *testing.T) {
	db := &recordingDB{rows: 1}

	welcomePost(t, db, "/account/welcome", settingsForm(),
		func(a *App, w http.ResponseWriter, r *http.Request) { a.CompleteOnboarding(w, r) })

	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}
	if got := db.calls[0].args[5]; got != true {
		t.Errorf("follow_turn = %#v, want true", got)
	}
	if got := db.calls[0].args[6]; got != true {
		t.Errorf("show_blood = %#v, want true", got)
	}
}





func TestNotNowStampsTheAccountAndStoresNothingElse(t *testing.T) {
	db := &recordingDB{rows: 1}

	
	
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

	
	
	trigger := rec.Header().Get("HX-Trigger")
	for _, want := range []string{"modal:close", "flash:toast"} {
		if !strings.Contains(trigger, want) {
			t.Errorf("HX-Trigger %q missing %s", trigger, want)
		}
	}
	if !strings.Contains(trigger, "gear") {
		t.Errorf("the toast does not say where to find the settings: %q", trigger)
	}
	
	if strings.Contains(trigger, "theme:change") {
		t.Errorf("a dismissal repainted the page: %q", trigger)
	}
}




func TestStorageReadsAsSomethingAPersonWouldSay(t *testing.T) {
	for _, c := range []struct {
		bytes int64
		want  string
	}{
		{0, "Nothing uploaded yet"},
		
		
		
		{-1, "Nothing uploaded yet"},
		{1, "1 B"},
		{1023, "1023 B"},
		
		
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024*1024 - 1, "1024.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{4*1024*1024*1024 + 512*1024*1024, "4.5 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
		
		
		{2048 * 1024 * 1024 * 1024 * 1024, "2048.0 TB"},
	} {
		if got := formatBytes(c.bytes); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}









func TestTheSaveSendsNoMarkupAddressedToTheHomepage(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Set("username", "Vex")
	rec := saveSettings(t, db, form)

	if body := rec.Body.String(); strings.Contains(body, "hx-swap-oob") || strings.Contains(body, `id="account-name"`) {
		t.Errorf("the reply carries the homepage's greeting:\n%s", body)
	}

	
	
	
	if !strings.Contains(rec.Body.String(), "errors-account-settings") {
		t.Errorf("the reply does not clear the error block:\n%s", rec.Body.String())
	}

	if got := settingsChange(t, rec)["name"]; got != "Vex" {
		t.Errorf("the new name did not reach the page: %#v", got)
	}
}






func TestTheSaveHandsTheTableBackTheSettingsItIsObeying(t *testing.T) {
	db := &recordingDB{rows: 1}

	form := settingsForm()
	form.Del("show_blood")
	rec := saveSettings(t, db, form)

	change := settingsChange(t, rec)
	if got := change["followTurn"]; got != true {
		t.Errorf("followTurn = %#v, want true", got)
	}
	if got := change["showBlood"]; got != false {
		t.Errorf("showBlood = %#v, want the false that was just saved", got)
	}
}



func settingsChange(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %q", rec.Header().Get("HX-Trigger"))
	}

	change, ok := events["settings:change"].(map[string]any)
	if !ok {
		t.Fatalf("settings:change detail = %#v, want an object", events["settings:change"])
	}

	return change
}
