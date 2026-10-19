package controllers

import (
	"database/sql/driver"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tabletopper/internal/queries"

	"github.com/oklog/ulid/v2"
)

var (
	orderFirst  = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMV01")
	orderSecond = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMV02")
	orderThird  = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMV03")
	orderFourth = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMV04")
)

func TestArrangeKeepsTheChosenOrderAndEveryStoredRow(t *testing.T) {
	stored := []ulid.ULID{orderFirst, orderSecond, orderThird, orderFourth}
	gone := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMV99")
	got := arrange(stored, []ulid.ULID{orderThird, gone, orderFirst, orderThird})
	want := []ulid.ULID{orderThird, orderFirst, orderSecond, orderFourth}
	if len(got) != len(want) {
		t.Fatalf("arranged %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arranged %v, want %v", got, want)
		}
	}
	if got := arrange(nil, []ulid.ULID{orderFirst}); len(got) != 0 {
		t.Errorf("a row nobody stored was given a place: %v", got)
	}
}
func storedOrder(ids ...ulid.ULID) roomAnswer {
	answer := roomAnswer{columns: []string{"id"}, values: []driver.Value{ids[0].Bytes()}}
	for _, id := range ids[1:] {
		answer.more = append(answer.more, []driver.Value{id.Bytes()})
	}
	return answer
}
func orderApp(answers ...roomAnswer) (*App, *roomDB) {
	stub := &roomDB{rows: 1, answers: answers}
	db := stub.db()
	return &App{DB: db, Queries: queries.New(db), Hub: nopHub{}}, stub
}
func orderForm(ids ...ulid.ULID) url.Values {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, id.String())
	}
	return url.Values{"ids": {strings.Join(parts, ",")}}
}
func assertPlaces(t *testing.T, db *roomDB, table string, want []ulid.ULID) []recordedCall {
	t.Helper()
	steps := db.recordedSteps()
	if len(steps) != len(want)+3 || steps[0] != "BEGIN" || !strings.Contains(steps[1], "FOR UPDATE") || steps[len(steps)-1] != "COMMIT" {
		t.Fatalf("steps = %q, want BEGIN, a locking read, %d updates, COMMIT", steps, len(want))
	}
	updates := db.recorded()[1:]
	for place, id := range want {
		update := updates[place]
		if !strings.Contains(update.query, "UPDATE "+table) || !strings.Contains(update.query, "SET sort_order = ?") {
			t.Fatalf("update %d is not a place on %s:\n%s", place, table, update.query)
		}
		if got := update.args[0]; got != int64(place) {
			t.Errorf("update %d wrote place %v, want %d", place, got, place)
		}
		if got, ok := boundRoomID(update.args[1]); !ok || got != id {
			t.Errorf("place %d went to %v, want %v", place, got, id)
		}
	}
	return updates
}
func TestOrderAttacksRewritesEveryPlaceInOneTransaction(t *testing.T) {
	app, db := orderApp(storedOrder(orderFirst, orderSecond, orderThird))
	rec := attackRequest(t, app.OrderAttacks, http.MethodPost, orderForm(orderThird, orderFirst), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	assertPlaces(t, db, "attacks", []ulid.ULID{orderThird, orderFirst, orderSecond})
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Attack order saved.") {
		t.Errorf("no toast: %q", rec.Header().Get("HX-Trigger"))
	}
	if rec.Body.Len() != 0 {
		t.Errorf("the reorder answered with markup: %s", rec.Body.String())
	}
}
func TestOrderSpellsStaysInsideItsLevel(t *testing.T) {
	app, db := orderApp(storedOrder(orderFirst, orderSecond))
	rec := spellRequest(t, app.OrderSpells, http.MethodPost, orderForm(orderSecond, orderFirst), "3", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	read := db.recorded()[0]
	if got := read.args[len(read.args)-1]; got != int64(3) {
		t.Errorf("the read took level %v, want 3", got)
	}
	for _, update := range assertPlaces(t, db, "spells", []ulid.ULID{orderSecond, orderFirst}) {
		if got := update.args[len(update.args)-1]; got != int64(3) {
			t.Errorf("an update reached level %v, want 3", got)
		}
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Spell order saved.") {
		t.Errorf("no toast: %q", rec.Header().Get("HX-Trigger"))
	}
}
func TestOrderWithNothingStoredIsNotFound(t *testing.T) {
	app, db := orderApp(roomAnswer{})
	rec := attackRequest(t, app.OrderAttacks, http.MethodPost, orderForm(orderFirst), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	for _, step := range db.recordedSteps() {
		if strings.Contains(step, "SET sort_order") {
			t.Errorf("a list nobody owns was written: %q", step)
		}
	}
}
func TestOrderRefusesWhatItCannotReadWithoutTouchingTheDatabase(t *testing.T) {
	for name, form := range map[string]url.Values{
		"no ids":       {},
		"only commas":  {"ids": {" , ,"}},
		"a broken id":  {"ids": {orderFirst.String() + ",not-an-id"}},
		"a stray word": {"ids": {"attacks"}},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := orderApp()
			rec := attackRequest(t, app.OrderAttacks, http.MethodPost, form, "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if steps := db.recordedSteps(); len(steps) != 0 {
				t.Errorf("ran %q, want nothing", steps)
			}
		})
	}
	app, db := orderApp()
	rec := spellRequest(t, app.OrderSpells, http.MethodPost, orderForm(orderFirst), "10", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("level 10 answered %d, want 404", rec.Code)
	}
	if steps := db.recordedSteps(); len(steps) != 0 {
		t.Errorf("level 10 ran %q, want nothing", steps)
	}
}
func TestNewAttacksAndSpellsGoToTheBottom(t *testing.T) {
	for file, c := range map[string]struct{ name, table, scope string }{
		"attacks.sql": {"InsertAttack", "attacks", ""},
		"spells.sql":  {"InsertSpell", "spells", "spells.level = sqlc.arg(level)"},
	} {
		source, err := os.ReadFile(filepath.Join("..", "..", "sql", file))
		if err != nil {
			t.Fatalf("cannot read the queries: %v", err)
		}
		body := string(source)
		start := strings.Index(body, "-- name: "+c.name+" ")
		if start < 0 {
			t.Fatalf("no %s query", c.name)
		}
		statement := body[start:]
		if end := strings.Index(statement[1:], "-- name:"); end >= 0 {
			statement = statement[:end+1]
		}
		for _, want := range []string{
			"COALESCE(MAX(" + c.table + ".sort_order) + 1, 0)",
			"GROUP BY characters.id",
			c.scope,
		} {
			if !strings.Contains(statement, want) {
				t.Errorf("%s does not put the new row last; it is missing %q:\n%s", c.name, want, statement)
			}
		}
	}
}
