package controllers
import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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
var testMonsterID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS4")
func createMonster(t *testing.T, app *App, form url.Values, picture []byte) *deadlineRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for field, values := range form {
		for _, value := range values {
			if err := writer.WriteField(field, value); err != nil {
				t.Fatalf("writing %s: %v", field, err)
			}
		}
	}
	filename := ""
	if picture != nil {
		filename = "picture.png"
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		t.Fatalf("writing the picture part: %v", err)
	}
	if _, err := part.Write(picture); err != nil {
		t.Fatalf("writing the picture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the form: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/monsters", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := newRecorder()
	app.NewMonsterForm(rec, r)
	return rec
}
func TestCreateMonsterRedirectsToTheEditor(t *testing.T) {
	app, db := newPanelApp(1)
	rec := createMonster(t, app, url.Values{"name": {"  Ancient Red Dragon  "}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	call := db.only(t)
	if !strings.Contains(call.query, "INSERT INTO monsters") {
		t.Errorf("statement is not the create: %q", call.query)
	}
	if name, ok := call.args[2].(string); !ok || name != "Ancient Red Dragon" {
		t.Errorf("stored name = %v, want %q", call.args[2], "Ancient Red Dragon")
	}
	if owner := call.args[1]; owner != testOwnerID {
		t.Errorf("owner = %v, want %v", owner, testOwnerID)
	}
	id, ok := call.args[0].(interface{ String() string })
	if !ok {
		t.Fatalf("first argument is not an id: %T", call.args[0])
	}
	if want := "/monsters/" + id.String() + "/edit"; rec.Header().Get("HX-Redirect") != want {
		t.Errorf("HX-Redirect = %q, want %q", rec.Header().Get("HX-Redirect"), want)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty -- the dialog is about to be navigated away from", body)
	}
	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}
	if events["flash:toast"] != "Ancient Red Dragon has been created." {
		t.Errorf("toast = %v", events["flash:toast"])
	}
}
func TestCreateMonsterRejectsBadNamesWithoutWriting(t *testing.T) {
	for _, c := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: "Name is required."},
		{name: "whitespace only", value: "   \t ", want: "Name is required."},
		{name: "too long", value: strings.Repeat("a", pages.MonsterNameLimit+1), want: "Name must be 128 characters or fewer."},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)
			rec := createMonster(t, app, url.Values{"name": {c.value}}, nil)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
			if rec.Header().Get("HX-Redirect") != "" {
				t.Error("a rejected create still sent HX-Redirect")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body missing %q: %s", c.want, body)
			}
			if !strings.Contains(rec.Body.String(), `id="errors-new-monster"`) {
				t.Errorf("body is not the error block: %s", rec.Body.String())
			}
		})
	}
}
func TestCreateMonsterMeasuresTheNameInCharactersNotBytes(t *testing.T) {
	app, db := newPanelApp(1)
	rec := createMonster(t, app, url.Values{"name": {strings.Repeat("é", pages.MonsterNameLimit)}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
}
func TestCreateMonsterCannotCarrySheetData(t *testing.T) {
	app, db := newPanelApp(1)
	block := url.Values{
		"name":                  {"Goblin"},
		"cr":                    {"30"},
		"ac":                    {"25"},
		"hp":                    {"697"},
		"str":                   {"30"},
		"dex":                   {"30"},
		"legendary_action_uses": {"5"},
		"senses":                {"Truesight 120 ft."},
		"languages":             {"All"},
		"description":           {"It is much stronger than it looks."},
	}
	rec := createMonster(t, app, block, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	call := db.only(t)
	if len(call.args) != 3 {
		t.Errorf("statement took %d values, want 3: %v", len(call.args), call.args)
	}
	if placeholders := strings.Count(call.query, "?"); placeholders != 3 {
		t.Errorf("statement has %d placeholders, want 3:\n%s", placeholders, call.query)
	}
	for _, arg := range call.args {
		if s, ok := arg.(string); ok && s != "Goblin" {
			t.Errorf("a form value other than the name reached the database: %q", s)
		}
	}
	if fields := reflect.TypeOf(queries.CreateMonsterFromNameParams{}).NumField(); fields != 3 {
		t.Errorf("CreateMonsterFromNameParams has %d fields, want 3 (id, owner, name)", fields)
	}
}
func TestCreateMonsterRefusesAPictureItCannotDecodeAndWritesNothing(t *testing.T) {
	app, db := newPanelApp(1)
	rec := createMonster(t, app, url.Values{"name": {"Goblin"}}, []byte("this is not an image"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(db.calls) != 0 {
		t.Errorf("the monster was created anyway: %v", db.calls)
	}
	if rec.Header().Get("HX-Redirect") != "" {
		t.Error("a rejected create still sent HX-Redirect")
	}
	if body := rec.Body.String(); !strings.Contains(body, "Only PNG, JPEG, and WEBP") {
		t.Errorf("body does not say what is wrong with the file: %s", body)
	}
	if !strings.Contains(rec.Body.String(), `id="errors-new-monster"`) {
		t.Errorf("body is not the dialog's error block: %s", rec.Body.String())
	}
}
func TestCreateMonsterTakesAnOrdinaryFormPost(t *testing.T) {
	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.NewMonsterForm, url.Values{"name": {"Goblin"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	call := db.only(t)
	if !strings.Contains(call.query, "INSERT INTO monsters") {
		t.Errorf("statement is not the create: %q", call.query)
	}
	if name, ok := call.args[2].(string); !ok || name != "Goblin" {
		t.Errorf("stored name = %v, want %q", call.args[2], "Goblin")
	}
}
func tablesHoldingMonsterRows(t *testing.T) []string {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}
	tables := []string{}
	definition := regexp.MustCompile("(?s)CREATE TABLE `([a-z_]+)` \\((.*?)\n\\) ENGINE=")
	for _, match := range definition.FindAllStringSubmatch(string(schema), -1) {
		if strings.Contains(match[2], "`monster_id`") {
			tables = append(tables, match[1])
		}
	}
	if len(tables) == 0 {
		t.Fatal("no table in db/schema.sql carries a monster_id")
	}
	return tables
}
const monsterShareTable = "shares"
func TestDeletingAMonsterEmptiesEveryTableThatHoldsItsRows(t *testing.T) {
	app, db := newPanelApp(1)
	if err := deleteMonsterRows(context.Background(), app.Queries, testMonsterID, testOwnerID); err != nil {
		t.Fatalf("deleteMonsterRows: %v", err)
	}
	emptied := map[string]bool{}
	for _, table := range deleteTargets(t, db.calls) {
		if emptied[table] {
			t.Errorf("%s is emptied twice", table)
		}
		emptied[table] = true
	}
	want := append([]string{monsterShareTable}, tablesHoldingMonsterRows(t)...)
	for _, table := range want {
		if !emptied[table] {
			t.Errorf("a monster delete leaves %s behind", table)
		}
		delete(emptied, table)
	}
	for table := range emptied {
		t.Errorf("a monster delete empties %s, which holds no rows of its own", table)
	}
}
func TestTheMonsterPurgeIsScopedToItsOwner(t *testing.T) {
	app, db := newPanelApp(1)
	if err := deleteMonsterRows(context.Background(), app.Queries, testMonsterID, testOwnerID); err != nil {
		t.Fatalf("deleteMonsterRows: %v", err)
	}
	if len(db.calls) == 0 {
		t.Fatal("the purge ran no statements at all")
	}
	for _, call := range db.calls {
		named := strings.Contains(call.query, "monster_id") || strings.Contains(call.query, "resource_id")
		if !strings.Contains(call.query, "owner_id") || !named {
			t.Errorf("a purge statement names one id and not both:\n%s", call.query)
		}
		found := map[ulid.ULID]bool{}
		for _, arg := range call.args {
			if id, ok := boundID(arg); ok {
				found[id] = true
			}
		}
		if !found[testMonsterID] || !found[testOwnerID] {
			t.Errorf("statement ran with %v, want both the monster and the owner:\n%s", call.args, call.query)
		}
	}
}
func monsterListRequest(t *testing.T, app *App, query string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/fragment/monster/list?"+query, nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.MonsterListFragment(rec, r)
	return rec
}
func TestMonsterListFragmentRefusesAnOverlongTerm(t *testing.T) {
	app, db := newPanelApp(1)
	rec := monsterListRequest(t, app, "q="+strings.Repeat("a", pages.MonsterNameLimit+1))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty -- http.NotFound would write a page-shaped one", body)
	}
	if len(db.calls) != 0 {
		t.Errorf("the overlong term was searched for anyway: %v", db.calls)
	}
	app, db = newPanelApp(1)
	monsterListRequest(t, app, "q="+strings.Repeat("a", pages.MonsterNameLimit))
	if len(db.calls) != 1 {
		t.Fatalf("a term at the limit ran %d statements, want 1", len(db.calls))
	}
}
func TestSearchingTheManualEscapesWhatLIKEWouldRead(t *testing.T) {
	app, db := newPanelApp(1)
	monsterListRequest(t, app, "")
	if call := db.only(t); !strings.Contains(call.query, "SELECT") || strings.Contains(call.query, "LIKE") {
		t.Errorf("an empty box searched instead of listing:\n%s", call.query)
	}
	app, db = newPanelApp(1)
	monsterListRequest(t, app, "q="+url.QueryEscape("100%_goblin"))
	call := db.only(t)
	if !strings.Contains(call.query, "LIKE") {
		t.Errorf("a term did not reach the search statement:\n%s", call.query)
	}
	if got := call.args[1]; got != `%100\%\_goblin%` {
		t.Errorf("term = %q, want the wildcards escaped and the whole thing wrapped", got)
	}
}
