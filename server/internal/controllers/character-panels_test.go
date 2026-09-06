package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// recordingDB stands in for the pool. It captures the one statement a panel
// handler runs, which is what these tests are about: not that a save happened,
// but that it touched nothing else.
type recordingDB struct {
	calls []recordedCall
	// reads is QueryRowContext only, kept apart from calls so that only(t) and
	// every column assertion in this file still see exactly the statement their
	// handler wrote. The four panels that refresh the derived values read the
	// character back afterwards, and counting that read as a call would have
	// made every one of those tests fail for a reason that is not about them.
	reads []recordedCall
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

// errNoRowsToGive is what every read gets back, because a *sql.Rows cannot be
// built outside database/sql. The statement is still recorded first, and that is
// what the read tests are about: not what came back, but what was sent -- which
// query, and which ids scope it.
var errNoRowsToGive = errors.New("recordingDB has no rows to give")

func (d *recordingDB) QueryContext(_ context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	d.calls = append(d.calls, recordedCall{query: query, args: args})
	if d.err != nil {
		return nil, d.err
	}

	return nil, errNoRowsToGive
}

// A *sql.Row, unlike a *sql.Rows, CAN be built outside database/sql: one comes
// back from any sql.DB, and a DB whose connector refuses to connect hands back a
// Row that answers Scan with that refusal. So the read is recorded and then
// fails, which is what the callers of it are written for -- finishDerivedPanel
// logs a failed read and returns, because the save it follows has already
// landed.
type refusingConnector struct{}

func (refusingConnector) Connect(context.Context) (driver.Conn, error) { return nil, errNoRowsToGive }

func (refusingConnector) Driver() driver.Driver { return nil }

var refusingDB = sql.OpenDB(refusingConnector{})

func (d *recordingDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	d.reads = append(d.reads, recordedCall{query: query, args: args})

	return refusingDB.QueryRowContext(ctx, query, args...)
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
			form:    url.Values{"xp": {"6500"}, "ac": {"17"}, "speed": {"30 ft."}, "initiative_bonus": {"2"}, "spellcasting_ability": {"wis"}, "spell_bonus_misc": {"1"}},
			want:    []string{"ac", "initiative_bonus", "level", "proficiency_bonus", "speed", "spell_bonus_misc", "spellcasting_ability", "xp"},
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
			form:       url.Values{"skills-stealth-misc": {"2"}, "skills-stealth-proficiency": {"expertise"}},
			pathValues: map[string]string{"kind": "skills"},
			want:       []string{"skill_proficiencies", "skills"},
		},
		{
			name:       "saving throws",
			handler:    func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses },
			form:       url.Values{"saving_throws-dex-misc": {"1"}, "saving_throws-dex-proficiency": {"proficient"}},
			pathValues: map[string]string{"kind": "saving_throws"},
			want:       []string{"saving_throw_proficiencies", "saving_throws"},
		},
		{
			name:    "features",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterFeatures },
			form:    url.Values{"features-name": {"Favored Enemy"}, "features-value": {"Undead"}},
			want:    []string{"features"},
		},
		{
			name:    "vitals",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterVitals },
			form:    url.Values{"max_hp": {"44"}, "current_hp": {"18"}, "temp_hp": {"0"}, "hit_dice": {"3d8"}, "hit_dice_spent": {"1"}, "death_save_successes": {"1", "1"}, "exhaustion": {"2"}, "heroic_inspiration": {"1"}},
			want:    []string{"current_hp", "death_save_failures", "death_save_successes", "exhaustion", "heroic_inspiration", "hit_dice", "hit_dice_spent", "max_hp", "temp_hp"},
		},
		{
			name:    "personality",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterPersonality },
			form:    url.Values{"personality_traits": {"Quick to laugh."}, "ideals": {"Freedom."}, "bonds": {"My old company."}, "flaws": {"Locked doors."}},
			want:    []string{"bonds", "flaws", "ideals", "personality_traits"},
		},
		{
			name:    "appearance",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAppearance },
			form:    url.Values{"age": {"27"}, "height": {"5 ft. 10 in."}, "weight": {"160 lb."}, "eyes": {"Grey"}, "skin": {"Sun-darkened"}, "hair": {"Black"}},
			want:    []string{"age", "eyes", "hair", "height", "skin", "weight"},
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
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterVitals }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterPersonality }},
		{handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAppearance }},
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

// The details panels are the only two whose columns a valid-looking value can
// overflow. MySQL runs in strict mode, so without the caps the driver would
// raise on the write and the player would get a 500 for overfilling a box the
// sheet handed them.
func TestDetailsPanelsRefuseAnOverlongValue(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		form    url.Values
		panel   string
		want    string
	}{
		{
			name:    "prose past the byte cap",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterPersonality },
			form:    url.Values{"bonds": {strings.Repeat("a", characterProseLimit+1)}},
			panel:   "personality",
			want:    "There is too much text in bonds.",
		},
		{
			name:    "a word past the character cap",
			handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAppearance },
			form:    url.Values{"hair": {strings.Repeat("a", characterWordLimit+1)}},
			panel:   "appearance",
			want:    "Hair must be 64 characters or fewer.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := panelPost(t, db, c.handler(app), c.form, map[string]string{"id": testCharacterID.String()})

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Error("an overlong value reached the column anyway")
			}
			body := rec.Body.String()
			if !strings.Contains(body, c.want) {
				t.Errorf("body = %q, want it to carry %q", body, c.want)
			}
			if !strings.Contains(body, `id="errors-`+c.panel+`"`) {
				t.Errorf("422 body is not the %s error block: %q", c.panel, body)
			}
		})
	}
}

// Each cap counts what its column counts, which only shows up in text that is
// not ASCII. VARCHAR(64) holds sixty-four CHARACTERS however many bytes they
// weigh, so a rune count is the right one there; TEXT holds BYTES, so a rune
// count would let a multibyte value through at several times the size.
func TestDetailCapsAreMeasuredInTheirColumnsOwnUnits(t *testing.T) {
	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.SaveCharacterAppearance, url.Values{
		"hair": {strings.Repeat("é", characterWordLimit)},
	}, map[string]string{"id": testCharacterID.String()})
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200: %d characters fit a VARCHAR(%d) whatever they weigh", rec.Code, characterWordLimit, characterWordLimit)
	}

	app, db = newPanelApp(1)
	rec = panelPost(t, db, app.SaveCharacterPersonality, url.Values{
		"ideals": {strings.Repeat("é", characterProseLimit)},
	}, map[string]string{"id": testCharacterID.String()})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422: %d two-byte runes is twice the byte cap", rec.Code, characterProseLimit)
	}
	if len(db.calls) != 0 {
		t.Error("a value twice the size of the cap reached the column")
	}
}

// The boxes carry a maxlength and the handlers carry a cap, and the two have to
// agree or one of them is decoration. Every VARCHAR(64) box on the page carries
// the column's own number, so that pair is exact: six of them are the appearance
// fields and the seventh is the hit dice pool. The textareas carry UTF-16 code
// units, which no encoding turns into more than three bytes each -- so a full
// box is still short of the byte cap, and only a request nobody's browser made
// finds it.
func TestTheCappedBoxesCannotOutrunTheirCaps(t *testing.T) {
	const proseMaxlength = 1024

	var buf bytes.Buffer
	if err := pages.EditCharacter(pages.EditCharacterPageData{CharacterID: testCharacterID.String()}).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	markup := buf.String()

	if got := strings.Count(markup, `maxlength="`+strconv.Itoa(characterWordLimit)+`"`); got != 7 {
		t.Errorf("word inputs carrying the column's maxlength = %d, want 7", got)
	}
	if got := strings.Count(markup, `maxlength="`+strconv.Itoa(proseMaxlength)+`"`); got != 4 {
		t.Errorf("prose boxes carrying maxlength=%d = %d, want 4", proseMaxlength, got)
	}
	if proseMaxlength*3 > characterProseLimit {
		t.Errorf("a full prose box is at most %d bytes and the cap is %d: a browser can now trip it", proseMaxlength*3, characterProseLimit)
	}
}

// The vitals bounds are rules rather than column widths, and the handler refuses
// rather than clamps: a sheet that stored something other than what was sent
// would be worse than one that says no. Each is also a CHECK on the table, so a
// value slipping past here is a 500 rather than a wrong row -- these are what
// keep the constraint unreachable.
func TestVitalsRefusesAValueOutsideTheRules(t *testing.T) {
	for _, c := range []struct {
		name string
		form url.Values
		want string
	}{
		{
			name: "a fourth death save success",
			form: url.Values{"death_save_successes": {"1", "1", "1", "1"}},
			want: "Death save successes must be between 0 and 3.",
		},
		{
			name: "a fourth death save failure",
			form: url.Values{"death_save_failures": {"1", "1", "1", "1"}},
			want: "Death save failures must be between 0 and 3.",
		},
		{
			name: "exhaustion past the level that kills",
			form: url.Values{"exhaustion": {"7"}},
			want: "Exhaustion must be between 0 and 6.",
		},
		{
			name: "more hit dice spent than a character can hold",
			form: url.Values{"hit_dice_spent": {"21"}},
			want: "Spent hit dice must be between 0 and 20.",
		},
		{
			name: "a hit dice pool wider than its column",
			form: url.Values{"hit_dice": {strings.Repeat("d", characterWordLimit+1)}},
			want: "Hit dice must be 64 characters or fewer.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := panelPost(t, db, app.SaveCharacterVitals, c.form, map[string]string{"id": testCharacterID.String()})

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
			if len(db.calls) != 0 {
				t.Error("the value was written anyway, and the CHECK constraint is now the only thing between it and the column")
			}
			if body := rec.Body.String(); !strings.Contains(body, c.want) {
				t.Errorf("body = %q, want it to carry %q", body, c.want)
			}
		})
	}
}

// Three of the six controls are checkboxes, and an unticked checkbox posts
// nothing at all -- so this handler reads values out of an absence, which is the
// shape every other panel handler is built to avoid. What makes it safe is that
// the panel posts all six together; these pin both readings of that.
func TestVitalsReadsItsCheckboxesFromWhatArrived(t *testing.T) {
	app, db := newPanelApp(1)
	panelPost(t, db, app.SaveCharacterVitals, url.Values{
		"max_hp": {"44"}, "current_hp": {"0"}, "temp_hp": {"0"},
		"hit_dice": {"3d8"}, "hit_dice_spent": {"0"}, "exhaustion": {"0"},
		"death_save_successes": {"1", "1"},
	}, map[string]string{"id": testCharacterID.String()})

	call := db.only(t)
	if got := writtenValue(t, call, "death_save_successes"); got != uint8(2) {
		t.Errorf("death save successes = %v, want 2: two boxes came back ticked", got)
	}
	if got := writtenValue(t, call, "death_save_failures"); got != uint8(0) {
		t.Errorf("death save failures = %v, want 0: no box came back", got)
	}
	if got := writtenValue(t, call, "heroic_inspiration"); got != false {
		t.Errorf("heroic inspiration = %v, want false: the box came back unticked", got)
	}

	app, db = newPanelApp(1)
	panelPost(t, db, app.SaveCharacterVitals, url.Values{
		"max_hp": {"44"}, "current_hp": {"0"}, "temp_hp": {"0"},
		"hit_dice": {"3d8"}, "hit_dice_spent": {"0"}, "exhaustion": {"0"},
		"death_save_failures": {"1", "1", "1"}, "heroic_inspiration": {"1"},
	}, map[string]string{"id": testCharacterID.String()})

	call = db.only(t)
	if got := writtenValue(t, call, "death_save_failures"); got != uint8(3) {
		t.Errorf("death save failures = %v, want 3", got)
	}
	if got := writtenValue(t, call, "heroic_inspiration"); got != true {
		t.Errorf("heroic inspiration = %v, want true", got)
	}
}

// writtenValue pulls one column's value out of a recorded UPDATE by name. sqlc
// binds the SET values in the order the statement names them, so the column's
// position in the SET clause is its argument's position -- which means a test
// can ask for "heroic_inspiration" rather than counting to it, and a column
// inserted ahead of it does not silently repoint the assertion at its
// neighbour. That is not hypothetical: moving the hit points into this panel
// shifted every index by three.
func writtenValue(t *testing.T, call recordedCall, column string) any {
	t.Helper()

	for i, name := range setColumns(t, call.query) {
		if name == column {
			return call.args[i]
		}
	}

	t.Fatalf("the statement does not write %q:\n%s", column, call.query)
	return nil
}

// The one piece of arithmetic every other derivation starts from, and the one
// most likely to be written the way the rules phrase it -- (score - 10) / 2 --
// which Go truncates toward zero. The odd scores below 10 are where that shows,
// and they are exactly the scores a dump stat lands on.
func TestAbilityModifierMatchesTheRules(t *testing.T) {
	for _, c := range []struct {
		score uint8
		want  int
	}{
		{1, -5}, {2, -4}, {7, -2}, {8, -1}, {9, -1}, {10, 0},
		{11, 0}, {12, 1}, {14, 2}, {15, 2}, {20, 5}, {30, 10},
	} {
		if got := abilityModifier(c.score); got != c.want {
			t.Errorf("abilityModifier(%d) = %d, want %d", c.score, got, c.want)
		}
	}
}

// Half proficiency rounds down, which is what the rules say and what a bard with
// a +3 proficiency bonus gets: one, not one and a half.
func TestProficiencyGrantRoundsHalfDown(t *testing.T) {
	for _, c := range []struct {
		state string
		bonus int
		want  int
	}{
		{pages.ProficiencyNone, 3, 0},
		{pages.ProficiencyHalf, 2, 1},
		{pages.ProficiencyHalf, 3, 1},
		{pages.ProficiencyHalf, 5, 2},
		{pages.ProficiencyProficient, 3, 3},
		{pages.ProficiencyExpertise, 3, 6},
		{"nonsense", 3, 0},
	} {
		if got := proficiencyGrant(c.state, c.bonus); got != c.want {
			t.Errorf("proficiencyGrant(%q, %d) = %d, want %d", c.state, c.bonus, got, c.want)
		}
	}
}

// testCharacter is a level 5 rogue-shaped sheet: Dex 16 (+3), Wis 13 (+1),
// Int 8 (-1), Cha 11 (+0), proficiency +3.
func testCharacter() queries.Character {
	return queries.Character{
		Str: 15, Dex: 16, Con: 14, Int: 8, Wis: 13, Cha: 11,
		ProficiencyBonus:         3,
		Skills:                   json.RawMessage(`{"stealth": 2, "arcana": 1}`),
		SkillProficiencies:       json.RawMessage(`{"stealth": "expertise", "perception": "proficient", "arcana": "half"}`),
		SavingThrows:             json.RawMessage(`{"dex": 1}`),
		SavingThrowProficiencies: json.RawMessage(`{"dex": "proficient"}`),
		SpellcastingAbility:      queries.CharactersSpellcastingAbilityNone,
	}
}

func derivedRow(t *testing.T, rows []pages.BonusRow, key string) pages.BonusRow {
	t.Helper()

	for _, row := range rows {
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("no row for %q", key)

	return pages.BonusRow{}
}

// A total is the ability modifier, plus what the proficiency state grants, plus
// the misc bonus -- and it is the whole reason the panel changed, so each of the
// three has to be visible in the answer.
func TestASkillTotalIsItsThreeParts(t *testing.T) {
	derived := characterDerived(testCharacter())

	for _, c := range []struct {
		key  string
		want string
		why  string
	}{
		{"stealth", "+11", "dex +3, expertise +6, misc +2"},
		{"perception", "+4", "wis +1, proficient +3, no misc"},
		{"arcana", "+1", "int -1, half proficiency +1, misc +1"},
		{"survival", "+1", "wis +1 and nothing else"},
		{"athletics", "+2", "str +2 and nothing else"},
		{"deception", "+0", "cha +0 and nothing else, and a zero bonus still carries its plus"},
	} {
		if got := derivedRow(t, derived.Skills, c.key).Total; got != c.want {
			t.Errorf("%s = %s, want %s (%s)", c.key, got, c.want, c.why)
		}
	}

	if got := derivedRow(t, derived.SavingThrows, "dex").Total; got != "+7" {
		t.Errorf("dex save = %s, want +7 (dex +3, proficient +3, misc +1)", got)
	}
	if got := derivedRow(t, derived.SavingThrows, "int").Total; got != "-1" {
		t.Errorf("int save = %s, want -1 (int -1 and nothing else)", got)
	}

	// The row also carries back what it was set from, or the controls would
	// render empty on every reload.
	stealth := derivedRow(t, derived.Skills, "stealth")
	if stealth.Proficiency != pages.ProficiencyExpertise || stealth.Misc != "2" {
		t.Errorf("stealth renders %q/%q, want expertise/2", stealth.Proficiency, stealth.Misc)
	}
}

// Ten plus the perception bonus, which means it moves when the Wisdom score
// does, when the proficiency does, and when experience crosses a level.
func TestPassivePerceptionIsTenPlusPerception(t *testing.T) {
	character := testCharacter()
	if got := characterDerived(character).PassivePerception; got != "14" {
		t.Errorf("passive perception = %s, want 14 (10 + wis +1 + proficient +3)", got)
	}

	character.Wis = 20
	if got := characterDerived(character).PassivePerception; got != "18" {
		t.Errorf("passive perception = %s, want 18 after the wisdom went up", got)
	}
}

// A character who casts nothing has no spell save DC, and the 8 the arithmetic
// would produce for one is a number a fighter would have to learn to ignore.
func TestSpellNumbersAreDashesUntilAnAbilityIsChosen(t *testing.T) {
	derived := characterDerived(testCharacter())
	if derived.SpellSaveDC != "—" || derived.SpellAttackBonus != "—" {
		t.Errorf("a non-caster reads %s / %s, want a dash for each", derived.SpellSaveDC, derived.SpellAttackBonus)
	}
}

// And a caster's DC is always eight plus their attack bonus, which is what makes
// one misc bonus enough to carry both.
func TestSpellSaveDCIsEightPlusTheAttackBonus(t *testing.T) {
	character := testCharacter()
	character.SpellcastingAbility = "dex"
	character.SpellBonusMisc = 1

	derived := characterDerived(character)
	if derived.SpellAttackBonus != "+7" {
		t.Errorf("spell attack = %s, want +7 (proficiency +3, dex +3, misc +1)", derived.SpellAttackBonus)
	}
	if derived.SpellSaveDC != "15" {
		t.Errorf("spell save DC = %s, want 15", derived.SpellSaveDC)
	}
}

// The marshaller walks the grid's own list instead of scanning the request, so a
// key nothing asked for cannot reach the column. The one it replaced took
// whatever followed the prefix, and a row written that way would have sat in the
// blob forever with no control able to reach it.
func TestABonusGridStoresOnlyTheRowsItAsked(t *testing.T) {
	app, db := newPanelApp(1)

	panelPost(t, db, app.SaveCharacterBonuses, url.Values{
		"skills-stealth-misc":        {"2"},
		"skills-stealth-proficiency": {"expertise"},
		"skills-flying-misc":         {"99"},
		"skills-flying-proficiency":  {"expertise"},
	}, map[string]string{"id": testCharacterID.String(), "kind": "skills"})

	call := db.only(t)
	misc := map[string]int{}
	if err := json.Unmarshal(call.args[0].(json.RawMessage), &misc); err != nil {
		t.Fatalf("misc payload is not JSON: %v", err)
	}
	states := map[string]string{}
	if err := json.Unmarshal(call.args[1].(json.RawMessage), &states); err != nil {
		t.Fatalf("proficiency payload is not JSON: %v", err)
	}

	if _, invented := misc["flying"]; invented {
		t.Error("a key nothing asked for reached the misc column")
	}
	if _, invented := states["flying"]; invented {
		t.Error("a key nothing asked for reached the proficiency column")
	}
	if misc["stealth"] != 2 || states["stealth"] != pages.ProficiencyExpertise {
		t.Errorf("stealth stored as %d/%q, want 2/expertise", misc["stealth"], states["stealth"])
	}
	if got := len(states); got != len(pages.SkillEntries()) {
		t.Errorf("stored %d proficiency states, want one per skill (%d)", got, len(pages.SkillEntries()))
	}
	if states["athletics"] != pages.ProficiencyNone {
		t.Errorf("a row the form did not carry stored %q, want none", states["athletics"])
	}
}

// A proficiency state the select could not have produced is normalised rather
// than refused, and it lands on none -- not on whatever proficiencyGrant makes
// of an unknown word.
func TestAnUnknownProficiencyStateBecomesNone(t *testing.T) {
	app, db := newPanelApp(1)

	panelPost(t, db, app.SaveCharacterBonuses, url.Values{
		"skills-stealth-proficiency": {"legendary"},
	}, map[string]string{"id": testCharacterID.String(), "kind": "skills"})

	states := map[string]string{}
	if err := json.Unmarshal(db.only(t).args[1].(json.RawMessage), &states); err != nil {
		t.Fatalf("proficiency payload is not JSON: %v", err)
	}
	if states["stealth"] != pages.ProficiencyNone {
		t.Errorf("stealth stored as %q, want none", states["stealth"])
	}
}

// EVERY panel on the character editor reads the character back, and the read is
// scoped to this user. That is what keeps the page it was saved from correct
// without a list of which panel changes what: the derived block and the bar are
// both rendered from the row that comes back, so a panel writing max_hp or a
// name refreshes the readings that follow from it whether or not anybody
// remembered it would.
//
// A handler missing from this list is a panel that saves correctly and leaves
// the page stale, which is the failure this test exists to make loud.
func TestEveryCharacterPanelRefreshesThePage(t *testing.T) {
	for _, c := range []struct {
		name       string
		handler    func(*App) http.HandlerFunc
		pathValues map[string]string
	}{
		{name: "identity", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterIdentity }},
		{name: "abilities", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAbilities }},
		{name: "core stats", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterCoreStats }},
		{name: "vitals", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterVitals }},
		{name: "proficiencies", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterProficiencies }},
		{name: "personality", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterPersonality }},
		{name: "appearance", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterAppearance }},
		{name: "features", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterFeatures }},
		{name: "skills", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses }, pathValues: map[string]string{"kind": "skills"}},
		{name: "saving throws", handler: func(a *App) http.HandlerFunc { return a.SaveCharacterBonuses }, pathValues: map[string]string{"kind": "saving_throws"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			pathValues := map[string]string{"id": testCharacterID.String()}
			for key, value := range c.pathValues {
				pathValues[key] = value
			}
			panelPost(t, db, c.handler(app), url.Values{"name": {"Vex"}, "size": {"medium"}}, pathValues)

			if len(db.reads) == 0 {
				t.Fatal("saved without reading the character back, so the page it was saved from is now stale")
			}

			read := db.reads[0]
			if !strings.Contains(read.query, "FROM characters") {
				t.Errorf("the refresh read something other than the character:\n%s", read.query)
			}
			if len(read.args) != 2 || read.args[0] != testCharacterID || read.args[1] != testOwnerID {
				t.Errorf("the refresh is not scoped to this user's character: %v", read.args)
			}
		})
	}
}
