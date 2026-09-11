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
	"github.com/oklog/ulid/v2"
)
var testAttackID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS2")
func attackRequest(t *testing.T, handler http.HandlerFunc, method string, form url.Values, attackID string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "/characters/attacks", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	if attackID != "" {
		r.SetPathValue("attackId", attackID)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	handler(rec, r)
	return rec
}
func fullAttackForm() url.Values {
	return url.Values{
		"name":         {"Longsword"},
		"attack_bonus": {"+7"},
		"damage":       {"1d8+3"},
		"damage_type":  {"Slashing"},
		"mastery":      {"Sap"},
		"notes":        {"Versatile (1d10)"},
	}
}
func TestSaveAttackWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)
	rec := attackRequest(t, app.SaveAttack, http.MethodPost, fullAttackForm(), testAttackID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	call := db.only(t)
	if !strings.Contains(call.query, "UPDATE attacks") {
		t.Errorf("a row save wrote something other than attacks:\n%s", call.query)
	}
	got := sortedColumns(t, call.query)
	want := []string{"attack_bonus", "damage", "damage_type", "mastery", "name", "notes"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("columns written = %v, want %v", got, want)
	}
}
func TestAttackSelectsNormaliseAnythingNotOnTheList(t *testing.T) {
	form := fullAttackForm()
	form.Set("damage_type", "Homebrew")
	form.Set("mastery", "Topple; DROP TABLE attacks")
	app, db := newPanelApp(1)
	rec := attackRequest(t, app.SaveAttack, http.MethodPost, form, testAttackID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: an unknown choice is corrected, not refused", rec.Code)
	}
	call := db.only(t)
	if got := writtenValue(t, call, "damage_type"); got != "" {
		t.Errorf("damage type = %q, want the empty member", got)
	}
	if got := writtenValue(t, call, "mastery"); got != "" {
		t.Errorf("mastery = %q, want the empty member", got)
	}
}
func TestOverlongAttackFieldsAreRejectedNotTruncated(t *testing.T) {
	for _, c := range []struct {
		field string
		size  int
		want  string
	}{
		{"name", attackNameLimit + 1, "Attack name must be 128 characters or fewer."},
		{"attack_bonus", attackBonusLimit + 1, "Attack bonus must be 32 characters or fewer."},
		{"damage", attackDamageLimit + 1, "Damage must be 64 characters or fewer."},
		{"notes", attackNotesLimit + 1, "Those notes are too long to save."},
	} {
		t.Run(c.field, func(t *testing.T) {
			form := fullAttackForm()
			form.Set(c.field, strings.Repeat("a", c.size))
			app, db := newPanelApp(1)
			rec := attackRequest(t, app.SaveAttack, http.MethodPost, form, testAttackID.String())
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Error("the overlong value was sent to the column anyway")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body = %q, want it to carry %q", body, c.want)
			}
		})
	}
}
func TestAddAttackCannotCarryAttackData(t *testing.T) {
	app, db := newPanelApp(0)
	rec := attackRequest(t, app.AddAttack, http.MethodPost, fullAttackForm(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	call := db.only(t)
	if len(call.args) != 3 {
		t.Errorf("statement took %d values, want 3: %v", len(call.args), call.args)
	}
	if placeholders := strings.Count(call.query, "?"); placeholders != 3 {
		t.Errorf("statement has %d placeholders, want 3:\n%s", placeholders, call.query)
	}
	for _, arg := range call.args {
		if _, ok := arg.(ulid.ULID); !ok {
			t.Errorf("a value that is not an id reached the insert: %#v", arg)
		}
	}
	if !strings.Contains(call.query, "FROM characters") {
		t.Errorf("the insert is not guarded by the characters row:\n%s", call.query)
	}
	if fields := reflect.TypeOf(queries.InsertAttackParams{}).NumField(); fields != 3 {
		t.Errorf("InsertAttackParams has %d fields, want 3 (attack, character, owner)", fields)
	}
}
func TestDeleteAttackAnswers200SoTheRowIsSwappedOut(t *testing.T) {
	app, db := newPanelApp(1)
	rec := attackRequest(t, app.DeleteAttack, http.MethodDelete, nil, testAttackID.String())
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(db.only(t).query, "DELETE FROM attacks") {
		t.Errorf("the delete did not empty attacks:\n%s", db.only(t).query)
	}
}
func TestMissingAttackRowIsAnAttack404(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{"a save", func(a *App) http.HandlerFunc { return a.SaveAttack }, http.MethodPost},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteAttack }, http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := newPanelApp(0)
			rec := attackRequest(t, c.handler(app), c.method, fullAttackForm(), testAttackID.String())
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "attack") {
				t.Errorf("the alert does not name the attack: %s", trigger)
			}
		})
	}
}
func TestUnparseableAttackIDTouchesNoDatabase(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{"a save", func(a *App) http.HandlerFunc { return a.SaveAttack }, http.MethodPost},
		{"a delete", func(a *App) http.HandlerFunc { return a.DeleteAttack }, http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)
			rec := attackRequest(t, c.handler(app), c.method, fullAttackForm(), "not-a-ulid")
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Errorf("an unparseable id reached the database: %v", db.calls)
			}
		})
	}
}
func TestEveryAttackQueryIsScopedToTheOwner(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "attacks.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}
	statements := regexp.MustCompile(`(?m)^-- name: (\w+)`).FindAllStringSubmatchIndex(string(source), -1)
	if len(statements) == 0 {
		t.Fatal("no named queries in sql/attacks.sql")
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
