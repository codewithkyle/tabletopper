package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var testActionID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS5")




func monsterActionRequest(t *testing.T, handler http.HandlerFunc, method string, kind string, form url.Values, actionID string) *httptest.ResponseRecorder {
	t.Helper()

	body := strings.NewReader(form.Encode())
	r := httptest.NewRequest(method, "/monsters/actions", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testMonsterID.String())
	r.SetPathValue("kind", kind)
	if actionID != "" {
		r.SetPathValue("actionId", actionID)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

func fullMonsterActionForm() url.Values {
	return url.Values{
		"name":        {"Bite"},
		"description": {"Melee Attack Roll: +4, reach 5 ft. Hit: 5 (1d6 + 2) piercing damage."},
	}
}




func TestAddMonsterActionCannotCarryActionData(t *testing.T) {
	app, db := newPanelApp(0)

	
	
	
	
	rec := monsterActionRequest(t, app.AddMonsterAction, http.MethodPost, pages.MonsterActionKindAction, fullMonsterActionForm(), "")
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
		switch value := arg.(type) {
		case ulid.ULID:
		case queries.MonsterActionsKind:
			if string(value) != pages.MonsterActionKindAction {
				t.Errorf("the insert was given the wrong section: %q", value)
			}
		default:
			t.Errorf("a value that is neither an id nor the kind reached the insert: %#v", arg)
		}
	}
	if !strings.Contains(call.query, "FROM monsters") {
		t.Errorf("the insert is not guarded by the monsters row:\n%s", call.query)
	}

	
	
	if fields := reflect.TypeOf(queries.InsertMonsterActionParams{}).NumField(); fields != 4 {
		t.Errorf("InsertMonsterActionParams has %d fields, want 4 (action, kind, monster, owner)", fields)
	}
}





func TestAnUnknownActionKindNeverBecomesAStatement(t *testing.T) {
	for _, c := range []struct {
		name     string
		handler  func(*App) http.HandlerFunc
		method   string
		actionID string
	}{
		{"an add", func(a *App) http.HandlerFunc { return a.AddMonsterAction }, http.MethodPost, ""},
		{"a save", func(a *App) http.HandlerFunc { return a.SaveMonsterAction }, http.MethodPost, testActionID.String()},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteMonsterAction }, http.MethodDelete, testActionID.String()},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, kind := range []string{"", "mythic_action", "Trait", "trait; DROP TABLE monster_actions"} {
				app, db := newPanelApp(1)

				rec := monsterActionRequest(t, c.handler(app), c.method, kind, fullMonsterActionForm(), c.actionID)
				if rec.Code != http.StatusNotFound {
					t.Errorf("kind %q: status = %d, want 404", kind, rec.Code)
				}
				if len(db.calls) != 0 {
					t.Errorf("kind %q reached the database: %v", kind, db.calls)
				}
			}
		})
	}
}




func TestDeleteMonsterActionAnswers200SoTheRowIsSwappedOut(t *testing.T) {
	app, db := newPanelApp(1)

	rec := monsterActionRequest(t, app.DeleteMonsterAction, http.MethodDelete, pages.MonsterActionKindTrait, nil, testActionID.String())
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "DELETE FROM monster_actions") {
		t.Errorf("the delete did not empty monster_actions:\n%s", call.query)
	}
	if !strings.Contains(call.query, "owner_id") || !strings.Contains(call.query, "monster_id") {
		t.Errorf("the delete is not scoped by both ids:\n%s", call.query)
	}
	
	
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "Trait deleted.") {
		t.Errorf("toast = %s", trigger)
	}
}




func TestMissingActionRowIsAnAction404(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{"a save", func(a *App) http.HandlerFunc { return a.SaveMonsterAction }, http.MethodPost},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteMonsterAction }, http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := newPanelApp(0)

			rec := monsterActionRequest(t, c.handler(app), c.method, pages.MonsterActionKindTrait, fullMonsterActionForm(), testActionID.String())
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "action") {
				t.Errorf("the alert does not name the action: %s", trigger)
			}
		})
	}
}



func TestUnparseableActionIDTouchesNoDatabase(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{"a save", func(a *App) http.HandlerFunc { return a.SaveMonsterAction }, http.MethodPost},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteMonsterAction }, http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := monsterActionRequest(t, c.handler(app), c.method, pages.MonsterActionKindTrait, fullMonsterActionForm(), "not-a-ulid")
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Errorf("an unparseable id reached the database: %v", db.calls)
			}
		})
	}
}



func TestSaveMonsterActionWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)

	rec := monsterActionRequest(t, app.SaveMonsterAction, http.MethodPost, pages.MonsterActionKindAction, fullMonsterActionForm(), testActionID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	call := db.only(t)
	if !strings.Contains(call.query, "UPDATE monster_actions") {
		t.Errorf("a row save wrote something other than monster_actions:\n%s", call.query)
	}

	got := sortedColumns(t, call.query)
	if strings.Join(got, ",") != "description,name" {
		t.Errorf("columns written = %v, want [description name]", got)
	}
}





func TestOverlongMonsterActionFieldsAreRejectedNotTruncated(t *testing.T) {
	for _, c := range []struct {
		field string
		value string
		want  string
	}{
		{"name", strings.Repeat("a", pages.MonsterActionNameLimit+1), "Name must be 128 characters or fewer."},
		{"description", strings.Repeat("a", pages.MonsterProseLimit+1), "That description is too long to save."},
		{"description", strings.Repeat("é", pages.MonsterProseLimit), "That description is too long to save."},
	} {
		form := fullMonsterActionForm()
		form.Set(c.field, c.value)

		app, db := newPanelApp(1)
		rec := monsterActionRequest(t, app.SaveMonsterAction, http.MethodPost, pages.MonsterActionKindAction, form, testActionID.String())

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", c.field, rec.Code)
		}
		if len(db.calls) != 0 {
			t.Errorf("%s: the overlong value was sent to the column anyway", c.field)
		}
		if body := rec.Body.String(); !strings.Contains(body, c.want) {
			t.Errorf("%s: body = %q, want it to carry %q", c.field, body, c.want)
		}
	}

	
	
	form := fullMonsterActionForm()
	form.Set("name", strings.Repeat("é", pages.MonsterActionNameLimit))

	app, db := newPanelApp(1)
	if rec := monsterActionRequest(t, app.SaveMonsterAction, http.MethodPost, pages.MonsterActionKindAction, form, testActionID.String()); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200: 128 accented letters fit the column", rec.Code)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1", len(db.calls))
	}
}









func TestAnActionSaveRedrawsTheStatBlock(t *testing.T) {
	for _, c := range []struct {
		name     string
		handler  func(*App) http.HandlerFunc
		method   string
		actionID string
	}{
		{"a save", func(a *App) http.HandlerFunc { return a.SaveMonsterAction }, http.MethodPost, testActionID.String()},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteMonsterAction }, http.MethodDelete, testActionID.String()},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			monsterActionRequest(t, c.handler(app), c.method, pages.MonsterActionKindLairAction, fullMonsterActionForm(), c.actionID)

			if len(db.reads) == 0 {
				t.Fatal("the row changed without the monster being read back, so the block beside it is now stale")
			}
			read := db.reads[0]
			if !strings.Contains(read.query, "FROM monsters") {
				t.Errorf("the redraw read something other than the monster:\n%s", read.query)
			}
			if len(read.args) != 2 || read.args[0] != testMonsterID || read.args[1] != testOwnerID {
				t.Errorf("the redraw is not scoped to this user's monster: %v", read.args)
			}
		})
	}
}









func TestAddMonsterActionReadsBackTheRowItMade(t *testing.T) {
	app, db := newPanelApp(1)

	monsterActionRequest(t, app.AddMonsterAction, http.MethodPost, pages.MonsterActionKindTrait, nil, "")

	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	read := db.reads[0]
	if !strings.Contains(read.query, "FROM monster_actions") {
		t.Errorf("the add read back something other than the row it made:\n%s", read.query)
	}
	if len(read.args) != 3 {
		t.Fatalf("the read-back took %d values, want 3 (action, monster, owner): %v", len(read.args), read.args)
	}
	if read.args[1] != testMonsterID || read.args[2] != testOwnerID {
		t.Errorf("the read-back is not scoped to this user's monster: %v", read.args)
	}
}
