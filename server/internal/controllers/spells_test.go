package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var testSpellID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS1")




func spellRequest(t *testing.T, handler http.HandlerFunc, method string, form url.Values, level string, spellID string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, "/characters/spells", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	r.SetPathValue("level", level)
	if spellID != "" {
		r.SetPathValue("spellId", spellID)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

func fullSpellForm() url.Values {
	return url.Values{
		"name":          {"Fireball"},
		"school":        {"Evocation"},
		"components":    {"V, S, M (a tiny ball of bat guano and sulfur)"},
		"casting_time":  {"Action"},
		"casting_range": {"150 feet"},
		"duration":      {"Instantaneous"},
		"description":   {"A bright streak flashes from your pointing finger."},
		"prepared":      {"1"},
	}
}





func TestSaveSpellWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)

	rec := spellRequest(t, app.SaveSpell, http.MethodPost, fullSpellForm(), "3", testSpellID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "UPDATE spells") {
		t.Fatalf("did not update spells:\n%s", call.query)
	}

	want := []string{
		"casting_range", "casting_time", "components", "description",
		"duration", "is_prepared", "name", "school",
	}
	if got := sortedColumns(t, call.query); !reflect.DeepEqual(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}

	
	
	
	for _, column := range setColumns(t, call.query) {
		if column == "level" {
			t.Error("a spell save writes its own level")
		}
	}

	
	
	
	
	scope := call.args[len(call.args)-4:]
	for i, want := range []ulid.ULID{testSpellID, testCharacterID, testOwnerID} {
		if got, ok := scope[i].(ulid.ULID); !ok || got != want {
			t.Errorf("scope[%d] = %v, want %v", i, scope[i], want)
		}
	}
	if got, ok := scope[3].(uint8); !ok || got != 3 {
		t.Errorf("scope level = %v, want 3", scope[3])
	}
}








func TestPreparedIsReadFromTheAbsenceOfTheField(t *testing.T) {
	for _, c := range []struct {
		name string
		form url.Values
		want bool
	}{
		{name: "ticked", form: fullSpellForm(), want: true},
		{name: "unticked", form: func() url.Values {
			f := fullSpellForm()
			f.Del("prepared")
			return f
		}(), want: false},
		{name: "empty form", form: url.Values{}, want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			spellRequest(t, app.SaveSpell, http.MethodPost, c.form, "3", testSpellID.String())

			call := db.only(t)
			
			
			
			got, ok := call.args[7].(bool)
			if !ok {
				t.Fatalf("is_prepared arg is %T, want bool", call.args[7])
			}
			if got != c.want {
				t.Errorf("prepared = %v, want %v", got, c.want)
			}
		})
	}
}




func TestSpellSchoolFallsBackRatherThanFailing(t *testing.T) {
	for _, c := range []struct{ posted, want string }{
		{"Abjuration", "Abjuration"},
		{"", pages.DefaultSpellSchool},
		{"Chronomancy", pages.DefaultSpellSchool},
		{"evocation", pages.DefaultSpellSchool},
	} {
		app, db := newPanelApp(1)

		form := fullSpellForm()
		form.Set("school", c.posted)
		spellRequest(t, app.SaveSpell, http.MethodPost, form, "3", testSpellID.String())

		if got := db.only(t).args[1]; got != c.want {
			t.Errorf("school %q stored as %v, want %v", c.posted, got, c.want)
		}
	}
}





func TestAddSpellCannotCarrySpellData(t *testing.T) {
	app, db := newPanelApp(0)

	
	
	
	
	rec := spellRequest(t, app.AddSpell, http.MethodPost, fullSpellForm(), "3", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	call := db.only(t)
	if len(call.args) != 4 {
		t.Errorf("statement took %d values, want 4: %v", len(call.args), call.args)
	}
	if placeholders := strings.Count(call.query, "?"); placeholders != 4 {
		t.Errorf("statement has %d placeholders, want 4:\n%s", placeholders, call.query)
	}
	for _, arg := range call.args {
		switch arg.(type) {
		case ulid.ULID, uint8:
		default:
			t.Errorf("a value that is not an id or a level reached the insert: %#v", arg)
		}
	}
	if !strings.Contains(call.query, "FROM characters") {
		t.Errorf("the insert is not guarded by the characters row:\n%s", call.query)
	}

	if fields := reflect.TypeOf(queries.InsertSpellParams{}).NumField(); fields != 4 {
		t.Errorf("InsertSpellParams has %d fields, want 4 (spell, level, character, owner)", fields)
	}
}






func TestDeleteSpellAnswers200SoTheRowIsSwappedOut(t *testing.T) {
	app, db := newPanelApp(1)

	rec := spellRequest(t, app.DeleteSpell, http.MethodDelete, nil, "3", testSpellID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d -- 204 is in noSwap and would strand the row", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "DELETE FROM spells") {
		t.Fatalf("did not delete from spells:\n%s", call.query)
	}
	for i, want := range []ulid.ULID{testSpellID, testCharacterID, testOwnerID} {
		if got, ok := call.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("arg[%d] = %v, want %v", i, call.args[i], want)
		}
	}
	if got, ok := call.args[3].(uint8); !ok || got != 3 {
		t.Errorf("arg[3] = %v, want level 3", call.args[3])
	}
}






func TestMissingSpellRowIsASpell404(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{name: "save", handler: func(a *App) http.HandlerFunc { return a.SaveSpell }, method: http.MethodPost},
		{name: "delete", handler: func(a *App) http.HandlerFunc { return a.DeleteSpell }, method: http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := newPanelApp(0)

			rec := spellRequest(t, c.handler(app), c.method, fullSpellForm(), "3", testSpellID.String())
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "spell") {
				t.Errorf("the alert does not name the spell: %s", trigger)
			}
		})
	}
}



func TestUnparseableSpellIDTouchesNoDatabase(t *testing.T) {
	app, db := newPanelApp(1)

	rec := spellRequest(t, app.SaveSpell, http.MethodPost, fullSpellForm(), "3", "not-a-ulid")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}






func TestSpellLevelIsBoundedToTheTen(t *testing.T) {
	for _, raw := range []string{"0", "1", "9"} {
		if _, ok := parseSpellLevel(raw); !ok {
			t.Errorf("level %q was refused", raw)
		}
	}
	for _, raw := range []string{"", " ", "10", "-1", "99", "3.5", "slots", "0x3", "+3"} {
		if level, ok := parseSpellLevel(raw); ok {
			t.Errorf("level %q was accepted as %d", raw, level)
		}
	}

	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
		spellID string
	}{
		{name: "add", handler: func(a *App) http.HandlerFunc { return a.AddSpell }, method: http.MethodPost},
		{name: "save", handler: func(a *App) http.HandlerFunc { return a.SaveSpell }, method: http.MethodPost, spellID: testSpellID.String()},
		{name: "delete", handler: func(a *App) http.HandlerFunc { return a.DeleteSpell }, method: http.MethodDelete, spellID: testSpellID.String()},
		{name: "slots", handler: func(a *App) http.HandlerFunc { return a.SaveSpellSlots }, method: http.MethodPost},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := spellRequest(t, c.handler(app), c.method, fullSpellForm(), "10", c.spellID)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}




func TestSlotSaveRefusesCantrips(t *testing.T) {
	app, db := newPanelApp(1)

	rec := spellRequest(t, app.SaveSpellSlots, http.MethodPost, url.Values{"slots": {"4"}, "used": {"1"}}, "0", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}





func TestSlotSaveWritesOnlyTheCounters(t *testing.T) {
	app, db := newPanelApp(1)

	rec := spellRequest(t, app.SaveSpellSlots, http.MethodPost, url.Values{"slots": {"4"}, "used": {"1"}}, "3", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "INSERT INTO spell_slots") {
		t.Fatalf("did not write spell_slots:\n%s", call.query)
	}
	if strings.Contains(call.query, "UPDATE characters") {
		t.Errorf("a slot save reaches the characters row:\n%s", call.query)
	}
	if !strings.Contains(call.query, "FROM characters") {
		t.Errorf("the upsert is not guarded by the characters row:\n%s", call.query)
	}

	
	
	
	if len(call.args) != 7 {
		t.Fatalf("statement took %d values, want 7: %v", len(call.args), call.args)
	}
	for i, want := range map[int]uint8{0: 3, 1: 4, 2: 1, 5: 4, 6: 1} {
		if got, ok := call.args[i].(uint8); !ok || got != want {
			t.Errorf("arg[%d] = %v, want %d", i, call.args[i], want)
		}
	}
	for i, want := range map[int]ulid.ULID{3: testCharacterID, 4: testOwnerID} {
		if got, ok := call.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("arg[%d] = %v, want %v", i, call.args[i], want)
		}
	}
}




func TestUsedIsCappedAtSlots(t *testing.T) {
	app, db := newPanelApp(1)

	spellRequest(t, app.SaveSpellSlots, http.MethodPost, url.Values{"slots": {"2"}, "used": {"9"}}, "3", "")

	call := db.only(t)
	if got, ok := call.args[2].(uint8); !ok || got != 2 {
		t.Errorf("used = %v, want it capped at 2", call.args[2])
	}
}





func TestSlotCountsAreCoercedNotRejected(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want uint8
	}{
		{raw: "", want: 0},
		{raw: "   ", want: 0},
		{raw: "not a number", want: 0},
		{raw: "0", want: 0},
		{raw: "-4", want: 0},
		{raw: "7", want: 7},
		{raw: "99", want: spellSlotLimit},
		{raw: "255", want: spellSlotLimit},
		{raw: "99999999999999", want: spellSlotLimit},
	} {
		if got := parseSlotCount(c.raw); got != c.want {
			t.Errorf("slots %q = %d, want %d", c.raw, got, c.want)
		}
	}
}




func TestOverlongSpellFieldsAreRejectedNotTruncated(t *testing.T) {
	for _, c := range []struct {
		field string
		limit int
	}{
		{field: "name", limit: spellNameLimit},
		{field: "components", limit: spellComponentsLimit},
		{field: "casting_time", limit: spellCastingTimeLimit},
		{field: "casting_range", limit: spellRangeLimit},
		{field: "duration", limit: spellDurationLimit},
	} {
		t.Run(c.field, func(t *testing.T) {
			app, db := newPanelApp(1)

			form := fullSpellForm()
			form.Set(c.field, strings.Repeat("é", c.limit+1))

			rec := spellRequest(t, app.SaveSpell, http.MethodPost, form, "3", testSpellID.String())
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}

	
	
	
	app, db := newPanelApp(1)
	form := fullSpellForm()
	form.Set("name", strings.Repeat("é", spellNameLimit))

	rec := spellRequest(t, app.SaveSpell, http.MethodPost, form, "3", testSpellID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1", len(db.calls))
	}
}






func TestTheCountersAreReadOneLevelAndTenLevelsAtATime(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "spells.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}
	body := string(source)

	statement := func(name string) string {
		start := strings.Index(body, "-- name: "+name)
		if start < 0 {
			t.Fatalf("no %s query", name)
		}
		out := body[start:]
		if end := strings.Index(out[1:], "-- name:"); end >= 0 {
			out = out[:end+1]
		}
		return out
	}

	if got := statement("GetSpellSlots"); !strings.Contains(got, "level = ?") {
		t.Errorf("GetSpellSlots is not one level:\n%s", got)
	}
	if got := statement("ListSpellSlots"); strings.Contains(got, "level = ?") {
		t.Errorf("ListSpellSlots is filtered to one level:\n%s", got)
	}
	if got := statement("ListSpellSlots"); !strings.Contains(got, "ORDER BY level") {
		t.Errorf("ListSpellSlots is not in level order:\n%s", got)
	}
	if got := statement("CountSpellsByLevel"); !strings.Contains(got, "GROUP BY level") {
		t.Errorf("CountSpellsByLevel does not group in SQL:\n%s", got)
	}
}





func TestMergeSpellLevelsBuildsAllTenFromWhateverCameBack(t *testing.T) {
	levels := mergeSpellLevels(
		[]queries.SpellSlot{{Level: 3, Slots: 4, Used: 1}},
		[]queries.CountSpellsByLevelRow{{Level: 0, Total: 5}, {Level: 3, Total: 2}},
	)

	if len(levels) != 10 {
		t.Fatalf("levels = %d, want 10", len(levels))
	}
	for i, level := range levels {
		if level.Level != i {
			t.Errorf("levels[%d] is level %d -- the slice is not in level order", i, level.Level)
		}
	}

	if got := levels[3]; got.Slots != "4" || got.Used != "1" || got.Count != 2 {
		t.Errorf("level 3 = %+v, want slots 4, used 1, count 2", got)
	}
	if got := levels[0]; got.Count != 5 {
		t.Errorf("cantrips count = %d, want 5", got.Count)
	}
	
	for _, level := range []int{1, 2, 4, 5, 6, 7, 8, 9} {
		if got := levels[level]; got.Slots != "0" || got.Used != "0" || got.Count != 0 {
			t.Errorf("untouched level %d = %+v, want zeroes", level, got)
		}
	}
}







func TestEverySpellQueryIsScopedToTheOwner(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "spells.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}

	statements := regexp.MustCompile(`(?m)^-- name: (\w+)`).FindAllStringSubmatchIndex(string(source), -1)
	if len(statements) == 0 {
		t.Fatal("no named queries in sql/spells.sql")
	}

	for i, at := range statements {
		name := string(source[at[2]:at[3]])
		end := len(source)
		if i+1 < len(statements) {
			end = statements[i+1][0]
		}
		body := string(source[at[0]:end])

		if !strings.Contains(body, "owner_id") {
			t.Errorf("%s is not scoped to the owner:\n%s", name, body)
		}
		
		
		
		if !strings.Contains(body, "character_id") && !strings.Contains(body, "characters") {
			t.Errorf("%s is not scoped to the character:\n%s", name, body)
		}
	}
}




func TestSpellsAreFilteredByLevelInSQL(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "spells.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}

	body := string(source)
	start := strings.Index(body, "-- name: ListSpellsAtLevel")
	if start < 0 {
		t.Fatal("no ListSpellsAtLevel query")
	}
	statement := body[start:]
	if end := strings.Index(statement[1:], "-- name:"); end >= 0 {
		statement = statement[:end+1]
	}

	if !strings.Contains(statement, "level = ?") {
		t.Errorf("the level page is not filtered in the statement:\n%s", statement)
	}
}




func TestSpellLevelTabsAreShortAndTheHeadingsAreNot(t *testing.T) {
	for level, want := range map[int]string{0: "Cantrips", 1: "1st", 2: "2nd", 3: "3rd", 4: "4th", 9: "9th"} {
		if got := pages.SpellLevelTab(level); got != want {
			t.Errorf("level %d tab = %q, want %q", level, got, want)
		}
	}
	for level, want := range map[int]string{0: "Cantrips", 1: "Level 1", 9: "Level 9"} {
		if got := pages.SpellLevelName(level); got != want {
			t.Errorf("level %d heading = %q, want %q", level, got, want)
		}
	}
}





func TestPreparedSpellGroupsFollowTheQueryOrder(t *testing.T) {
	groups := preparedSpellGroups([]queries.Spell{
		{ID: ulid.Make(), Level: 0, Name: "Fire Bolt", School: "Evocation"},
		{ID: ulid.Make(), Level: 0, Name: "Light", School: "Evocation"},
		{ID: ulid.Make(), Level: 1, Name: "Shield", School: "Abjuration"},
		{ID: ulid.Make(), Level: 3, Name: "Fireball", School: "Evocation"},
		{ID: ulid.Make(), Level: 3, Name: "Counterspell", School: "Abjuration"},
	})

	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(groups))
	}
	for i, want := range []struct {
		level  int
		name   string
		spells int
	}{
		{0, "Cantrips", 2},
		{1, "Level 1", 1},
		{3, "Level 3", 2},
	} {
		got := groups[i]
		if got.Level != want.level || got.Name != want.name || len(got.Spells) != want.spells {
			t.Errorf("groups[%d] = level %d %q with %d spells, want level %d %q with %d",
				i, got.Level, got.Name, len(got.Spells), want.level, want.name, want.spells)
		}
	}

	
	
	if groups[0].Spells[0].Name != "Fire Bolt" || groups[2].Spells[1].Name != "Counterspell" {
		t.Errorf("rows did not keep their order within a level: %+v", groups)
	}

	
	if got := preparedSpellGroups(nil); len(got) != 0 {
		t.Errorf("an empty result made %d groups", len(got))
	}
}





func TestPreparedQueryFiltersAndOrdersInSQL(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "spells.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}

	body := string(source)
	start := strings.Index(body, "-- name: ListPreparedSpells")
	if start < 0 {
		t.Fatal("no ListPreparedSpells query")
	}
	statement := body[start:]
	if end := strings.Index(statement[1:], "-- name:"); end >= 0 {
		statement = statement[:end+1]
	}

	if !strings.Contains(statement, "is_prepared = TRUE") {
		t.Errorf("the prepared view is not filtered in the statement:\n%s", statement)
	}
	if !strings.Contains(statement, "ORDER BY level") {
		t.Errorf("the prepared view is not ordered by level:\n%s", statement)
	}
}




func TestSpellsRootRedirectsToCantrips(t *testing.T) {
	app, db := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/characters/x/edit/spells", nil)
	r.SetPathValue("id", testCharacterID.String())
	rec := httptest.NewRecorder()
	app.CharacterSpellsRedirect(rec, r)

	if want := "/characters/" + testCharacterID.String() + "/edit/spells/0"; rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	
	
	
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}




func TestSpellsRootWillNotRedirectToAnUnparsedID(t *testing.T) {
	app, _ := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/characters/x/edit/spells", nil)
	r.SetPathValue("id", "https:
	rec := httptest.NewRecorder()
	app.CharacterSpellsRedirect(rec, r)

	if got := rec.Header().Get("Location"); got != "/characters" {
		t.Errorf("Location = %q, want /characters", got)
	}
}
