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
type recordingDB struct {
	calls []recordedCall
	reads []recordedCall
	rows  int64
	err   error
}
type recordedCall struct {
	query string
	args  []any
}
func boundID(arg any) (ulid.ULID, bool) {
	switch id := arg.(type) {
	case ulid.ULID:
		return id, true
	case *ulid.ULID:
		if id == nil {
			return ulid.ULID{}, false
		}
		return *id, true
	default:
		return ulid.ULID{}, false
	}
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
var errNoRowsToGive = errors.New("recordingDB has no rows to give")
func (d *recordingDB) QueryContext(_ context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	d.calls = append(d.calls, recordedCall{query: query, args: args})
	if d.err != nil {
		return nil, d.err
	}
	return nil, errNoRowsToGive
}
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
func tableColumns(t *testing.T, table string) []string {
	t.Helper()
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("cannot read the schema: %v", err)
	}
	body := regexp.MustCompile("(?s)CREATE TABLE `" + table + "` \\((.*?)\n\\) ENGINE=").FindSubmatch(schema)
	if body == nil {
		t.Fatalf("no %s table in db/schema.sql", table)
	}
	columns := []string{}
	for _, line := range strings.Split(string(body[1]), "\n") {
		match := regexp.MustCompile("^\\s+`([a-z_]+)` \\S").FindStringSubmatch(line)
		if match != nil {
			columns = append(columns, match[1])
		}
	}
	return columns
}
var unownedColumns = map[string]bool{
	"id":         true,
	"owner_id":   true,
	"asset_id":   true,
	"created_at": true,
	"updated_at": true,
}
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
		panelPost(t, db, panel.handler(app), url.Values{"name": {"Vex"}, "size": {"medium"}}, pathValues)
		for _, column := range sortedColumns(t, db.only(t).query) {
			if covered[column] {
				t.Errorf("column %q is written by two panels", column)
			}
			covered[column] = true
		}
	}
	for _, column := range tableColumns(t, "characters") {
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
func TestCoreStatsDerivesLevelAndProficiencyFromXP(t *testing.T) {
	app, db := newPanelApp(1)
	panelPost(t, db, app.SaveCharacterCoreStats, url.Values{
		"xp": {"48000"}, 
	}, map[string]string{"id": testCharacterID.String()})
	call := db.only(t)
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
		"name": {"   "}, 
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
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-`+panel+`" hidden></div>` {
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
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}
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
	stealth := derivedRow(t, derived.Skills, "stealth")
	if stealth.Proficiency != pages.ProficiencyExpertise || stealth.Misc != "2" {
		t.Errorf("stealth renders %q/%q, want expertise/2", stealth.Proficiency, stealth.Misc)
	}
}
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
func TestSpellNumbersAreDashesUntilAnAbilityIsChosen(t *testing.T) {
	derived := characterDerived(testCharacter())
	if derived.SpellSaveDC != "—" || derived.SpellAttackBonus != "—" {
		t.Errorf("a non-caster reads %s / %s, want a dash for each", derived.SpellSaveDC, derived.SpellAttackBonus)
	}
}
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
func TestOnlySkillsPrintTheAbilityTheyKeyOff(t *testing.T) {
	derived := characterDerived(testCharacter())
	for _, c := range []struct{ key, want string }{
		{key: "acrobatics", want: "DEX"},
		{key: "arcana", want: "INT"},
		{key: "athletics", want: "STR"},
	} {
		if got := derivedRow(t, derived.Skills, c.key).Abbr; got != c.want {
			t.Errorf("skill %s prints %q, want %q", c.key, got, c.want)
		}
	}
	for _, key := range []string{"str", "dex", "con", "int", "wis", "cha"} {
		if got := derivedRow(t, derived.SavingThrows, key).Abbr; got != "" {
			t.Errorf("saving throw %s prints %q under its own name", key, got)
		}
	}
}
