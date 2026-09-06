package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// recordingDB stands in for the pool. It captures the one statement a panel
// handler runs, which is what these tests are about: not that a save happened,
// but that it touched nothing else.
type recordingDB struct {
	calls []recordedCall
	rows  int64
	err   error
}

type recordedCall struct {
	query string
	args  []any
}

func (d *recordingDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	d.calls = append(d.calls, recordedCall{query: query, args: args})
	if d.err != nil {
		return nil, d.err
	}

	return fakeResult{rows: d.rows}, nil
}

func (d *recordingDB) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	panic("not used")
}

func (d *recordingDB) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	panic("not used")
}

func (d *recordingDB) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	panic("not used")
}

type fakeResult struct{ rows int64 }

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return r.rows, nil }

var (
	testCharacterID = ulid.MustParse("01ARZ3NDEKTSV4RRFFQ69G5FAV")
	testOwnerID     = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVRZ")
)

// panelPost drives one handler and hands back the recorder and the statements
// it ran. pathValues carries the {id} every panel route has plus whichever of
// {kind}/{field} the route under test declares.
func panelPost(t *testing.T, db *recordingDB, handler http.HandlerFunc, form url.Values, pathValues map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/characters/save", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for key, value := range pathValues {
		r.SetPathValue(key, value)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

func newPanelApp(rows int64) (*App, *recordingDB) {
	db := &recordingDB{rows: rows}
	return &App{Queries: queries.New(db)}, db
}

// setColumns pulls the column names out of the SET clause of an UPDATE, in the
// order the statement writes them, so a test can say what a statement wrote
// rather than matching the whole string. sqlc emits SET on its own line for the
// multi-column queries and inline for the single-column ones, hence the regexp
// rather than an index.
var setClause = regexp.MustCompile(`(?is)\bSET\b(.*?)\bWHERE\b`)

func setColumns(t *testing.T, query string) []string {
	t.Helper()

	match := setClause.FindStringSubmatch(query)
	if match == nil {
		t.Fatalf("not an UPDATE with a WHERE: %q", query)
	}

	columns := []string{}
	for _, assignment := range strings.Split(match[1], ",") {
		name, _, found := strings.Cut(assignment, "=")
		if !found {
			continue
		}
		name = strings.Trim(strings.TrimSpace(name), "`")
		if name != "" {
			columns = append(columns, name)
		}
	}

	return columns
}

// sortedColumns is setColumns for the comparisons that care about the set and
// not the order.
func sortedColumns(t *testing.T, query string) []string {
	t.Helper()

	columns := setColumns(t, query)
	sort.Strings(columns)

	return columns
}

func (d *recordingDB) only(t *testing.T) recordedCall {
	t.Helper()

	if len(d.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(d.calls))
	}

	return d.calls[0]
}

// THE REGRESSION TEST.
//
// The parse helpers return their fallback on an empty string rather than an
// error, so a handler reading fields its panel does not render would not fail
// -- it would write 10 over every ability score, 1 over the hit points and
// empty JSON over all six blobs, and report success. This asserts the Identity
// panel cannot reach any of those columns.
func TestIdentityPanelWritesOnlyIdentityColumns(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.SaveCharacterIdentity, url.Values{
		"name":       {"Vex"},
		"race":       {"Half-Elf"},
		"background": {"Outlander"},
		"alignment":  {"chaotic good"},
		"classes":    {"Ranger 5"},
		"size":       {"medium"},
	}, map[string]string{"id": testCharacterID.String()})

	call := db.only(t)
	got := sortedColumns(t, call.query)
	want := []string{"alignment", "background", "classes", "name", "race", "size"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("columns written = %v, want %v", got, want)
	}

	for _, forbidden := range []string{"str", "dex", "con", "int", "wis", "cha", "max_hp", "current_hp", "ac", "xp", "level", "skills", "saving_throws", "features"} {
		for _, column := range got {
			if column == forbidden {
				t.Errorf("identity save touched %q", forbidden)
			}
		}
	}

	// name, race, background, alignment, classes, size, id, owner_id.
	if len(call.args) != 8 {
		t.Errorf("args = %d, want 8", len(call.args))
	}
	if call.args[0] != "Vex" {
		t.Errorf("args[0] = %v, want Vex", call.args[0])
	}
	if call.args[len(call.args)-1] != testOwnerID {
		t.Errorf("owner scoping missing; last arg = %v", call.args[len(call.args)-1])
	}

	assertSavedToast(t, rec, "identity", "Identity saved.")
}

// Every panel is checked the same way, so the split cannot quietly widen.
func TestPanelsWriteOnlyTheirOwnColumns(t *testing.T) {
	cases := []struct {
		name       string
		handler    func(*App) http.HandlerFunc
		form       url.Values
		pathValues map[string]string
		want       []string
	}{
		{
			name:    "abilities",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAbilities },
			form:    url.Values{"str": {"16"}, "dex": {"14"}, "con": {"13"}, "int": {"10"}, "wis": {"12"}, "cha": {"8"}},
			want:    []string{"cha", "con", "dex", "int", "str", "wis"},
		},
		{
			name:    "core stats",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterCoreStats },
			form:    url.Values{"xp": {"6500"}, "ac": {"17"}, "max_hp": {"44"}, "current_hp": {"44"}, "temp_hp": {"0"}, "speed": {"30 ft."}, "initiative_bonus": {"2"}, "spell_save_dc": {"14"}, "spell_atk_bonus": {"6"}},
			want:    []string{"ac", "current_hp", "initiative_bonus", "level", "max_hp", "proficiency_bonus", "speed", "spell_atk_bonus", "spell_save_dc", "temp_hp", "xp"},
		},
		{
			name:    "proficiencies",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterProficiencies },
			form:    url.Values{"languages": {"Common, Elvish"}, "proficiencies": {"Thieves' tools"}},
			want:    []string{"languages", "proficiencies"},
		},
		{
			name:       "skills",
			handler:    func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses },
			form:       url.Values{"skills-stealth": {"7"}},
			pathValues: map[string]string{"kind": "skills"},
			want:       []string{"skills"},
		},
		{
			name:       "saving throws",
			handler:    func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses },
			form:       url.Values{"saving_throws-dex": {"5"}},
			pathValues: map[string]string{"kind": "saving_throws"},
			want:       []string{"saving_throws"},
		},
		{
			name:    "features",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterFeatures },
			form:    url.Values{"features-name": {"Favored Enemy"}, "features-value": {"Undead"}},
			want:    []string{"features"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			pathValues := map[string]string{"id": testCharacterID.String()}
			for key, value := range c.pathValues {
				pathValues[key] = value
			}

			rec := panelPost(t, db, c.handler(app), c.form, pathValues)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}

			got := sortedColumns(t, db.only(t).query)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("columns written = %v, want %v", got, c.want)
			}
		})
	}
}

// characterColumns reads the characters table out of the dumped schema. The
// test below compares the panels against it, so a column added to the table
// with no panel to write it fails rather than going unnoticed.
func characterColumns(t *testing.T) []string {
	t.Helper()

	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}

	body := regexp.MustCompile("(?s)CREATE TABLE `characters` \\((.*?)\n\\) ENGINE=").FindSubmatch(schema)
	if body == nil {
		t.Fatal("no characters table in db/schema.sql")
	}

	columns := []string{}
	for _, line := range strings.Split(string(body[1]), "\n") {
		// Column lines open with a backticked name and a type; the key and
		// constraint lines that follow them open with a keyword.
		match := regexp.MustCompile("^\\s+`([a-z_]+)` \\S").FindStringSubmatch(line)
		if match != nil {
			columns = append(columns, match[1])
		}
	}

	return columns
}

// Columns no panel is meant to write. The first three are the row's identity
// and its avatar, which POST /characters/{id}/avatar owns; the timestamps are
// the database's.
var unownedColumns = map[string]bool{
	"id":         true,
	"owner_id":   true,
	"asset_id":   true,
	"created_at": true,
	"updated_at": true,
}

// Every editable column belongs to exactly one panel. This is the invariant the
// split has to hold: a column two panels write races itself under a debounce,
// and a column no panel writes cannot be edited at all.
func TestPanelsCoverEveryEditableColumn(t *testing.T) {
	covered := map[string]bool{}
	panels := []struct {
		handler    func(*App) http.HandlerFunc
		pathValues map[string]string
	}{
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterIdentity }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAbilities }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterCoreStats }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterProficiencies }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses }, pathValues: map[string]string{"kind": "skills"}},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses }, pathValues: map[string]string{"kind": "saving_throws"}},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterFeatures }},
	}

	for _, panel := range panels {
		app, db := newPanelApp(1)

		pathValues := map[string]string{"id": testCharacterID.String()}
		for key, value := range panel.pathValues {
			pathValues[key] = value
		}

		// name and size are required; every other panel ignores them.
		panelPost(t, db, panel.handler(app), url.Values{"name": {"Vex"}, "size": {"medium"}}, pathValues)
		for _, column := range sortedColumns(t, db.only(t).query) {
			if covered[column] {
				t.Errorf("column %q is written by two panels", column)
			}
			covered[column] = true
		}
	}

	for _, column := range characterColumns(t) {
		if unownedColumns[column] {
			if covered[column] {
				t.Errorf("column %q is not meant to be editable and a panel writes it", column)
			}
			continue
		}
		if !covered[column] {
			t.Errorf("column %q is editable and no panel writes it", column)
		}
	}
}

// XP is the only field the user sets; level and proficiency follow from it and
// are written in the same statement so the row can never disagree with itself.
func TestCoreStatsDerivesLevelAndProficiencyFromXP(t *testing.T) {
	app, db := newPanelApp(1)

	panelPost(t, db, app.SaveCharacterCoreStats, url.Values{
		"xp": {"48000"}, // level 9
	}, map[string]string{"id": testCharacterID.String()})

	call := db.only(t)

	// The args are read positionally below, so pin the order that licenses it.
	if got := setColumns(t, call.query)[:3]; strings.Join(got, ",") != "xp,level,proficiency_bonus" {
		t.Fatalf("SET opens with %v, not xp,level,proficiency_bonus", got)
	}
	if got := call.args[0]; got != uint32(48000) {
		t.Errorf("xp = %v, want 48000", got)
	}
	if got := call.args[1]; got != uint8(9) {
		t.Errorf("level = %v, want 9", got)
	}
	if got := call.args[2]; got != uint16(4) {
		t.Errorf("proficiency bonus = %v, want 4", got)
	}
}

// Bonuses is the only panel left that takes its name from the path, and the
// segment decides both a column and a field-name prefix -- so an unchecked value
// would reach a query. The repeaters were tested here too until inventory left
// one of them, which now has a route of its own and no segment to get wrong.
func TestBonusPanelRejectsAnUnknownKind(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.SaveCharacterBonuses, url.Values{},
		map[string]string{"id": testCharacterID.String(), "kind": "spell_slots"})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Errorf("an unknown segment reached the database: %q", db.calls[0].query)
	}
}

func TestPanelRejectsAnUnparseableCharacterID(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.SaveCharacterIdentity, url.Values{"name": {"Vex"}, "size": {"medium"}},
		map[string]string{"id": "not-a-ulid"})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Error("a bad id reached the database")
	}
}

// Found-rows semantics: zero matched means the character is not this user's,
// not that the save was a no-op. It must not answer with a toast.
func TestPanelAnswers404WhenNoRowMatched(t *testing.T) {
	app, db := newPanelApp(0)

	rec := panelPost(t, db, app.SaveCharacterIdentity, url.Values{"name": {"Vex"}, "size": {"medium"}},
		map[string]string{"id": testCharacterID.String()})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Header().Get("HX-Trigger"), "flash:toast") {
		t.Error("a character that is not the caller's was reported as saved")
	}
}

func TestPanelValidationFailsBeforeTheWrite(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.SaveCharacterIdentity, url.Values{
		"name": {"   "}, // required, and whitespace does not count
		"size": {"medium"},
	}, map[string]string{"id": testCharacterID.String()})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if len(db.calls) != 0 {
		t.Error("an invalid panel was written anyway")
	}
	if body := rec.Body.String(); !strings.Contains(body, `id="errors-identity"`) {
		t.Errorf("422 body is not the identity error block: %q", body)
	}
}

// The repeaters carry no index; the set of rows in the post is the set of rows
// that will exist. That is what makes a deletion persist, so it is worth
// pinning: posting one row after two replaces the column with one row.
func TestRepeaterSaveReplacesTheWholeColumn(t *testing.T) {
	app, db := newPanelApp(1)

	panelPost(t, db, app.SaveCharacterFeatures, url.Values{
		"features-name":  {"Favored Enemy"},
		"features-value": {"Undead"},
	}, map[string]string{"id": testCharacterID.String()})

	call := db.only(t)
	var rows []map[string]string
	if err := json.Unmarshal(call.args[0].(json.RawMessage), &rows); err != nil {
		t.Fatalf("features payload is not JSON: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Favored Enemy" {
		t.Errorf("rows = %v, want the one posted row", rows)
	}
}

func assertSavedToast(t *testing.T, rec *httptest.ResponseRecorder, panel string, want string) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	// A save answers with the panel's error block, empty. That is what clears a
	// message an earlier save left there; 204 would leave it on screen over a
	// panel that has since saved.
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-`+panel+`"></div>` {
		t.Errorf("body = %q, want the cleared error block", body)
	}

	var events map[string]string
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}
	if got := events["flash:toast"]; got != want {
		t.Errorf("toast = %q, want %q", got, want)
	}
}

// The error block is the swap target for both replies, so a save that lands
// wipes the complaint the previous one left. This is the whole reason a
// successful panel save carries a body instead of answering 204.
func TestSuccessfulSaveClearsAnEarlierPanelError(t *testing.T) {
	app, db := newPanelApp(1)

	rejected := panelPost(t, db, app.SaveCharacterIdentity, url.Values{"name": {"  "}, "size": {"medium"}},
		map[string]string{"id": testCharacterID.String()})
	if !strings.Contains(rejected.Body.String(), "Name is required.") {
		t.Fatalf("the rejected save did not report the error: %q", rejected.Body.String())
	}

	accepted := panelPost(t, db, app.SaveCharacterIdentity, url.Values{"name": {"Vex"}, "size": {"medium"}},
		map[string]string{"id": testCharacterID.String()})
	if strings.Contains(accepted.Body.String(), "Name is required.") {
		t.Errorf("the error survived a successful save: %q", accepted.Body.String())
	}
	assertSavedToast(t, accepted, "identity", "Identity saved.")
}

// The add-row fragment reads nothing off the request. It used to take a ?field=
// that decided the name attributes on the row it handed back, which is why it
// had to check that value against an allowlist -- an unvalidated one would have
// put arbitrary field names into the next post. There is one repeater now, so
// the parameter is gone; this pins that a leftover one cannot change the answer.
func TestFeatureRowFragmentIgnoresEverythingOnTheRequest(t *testing.T) {
	app, db := newPanelApp(1)

	for _, query := range []string{"", "?field=weapons", "?field=traits"} {
		r := httptest.NewRequest(http.MethodGet, "/fragment/character/feature-row"+query, nil)
		r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
		rec := httptest.NewRecorder()
		app.FeatureRowFragment(rec, r)

		if rec.Code != http.StatusOK {
			t.Fatalf("%q: status = %d, want %d", query, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		for _, want := range []string{`name="features-name"`, `name="features-value"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%q: row is missing %s\n%s", query, want, body)
			}
		}
		for _, forbidden := range []string{"weapons", "traits"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%q: the query reached the markup (%s)\n%s", query, forbidden, body)
			}
		}
	}

	// A blank row is the same for every user and every character.
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}
