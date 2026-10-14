package controllers

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"tabletopper/internal/dnd"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
)

func coinRequest(t *testing.T, handler http.HandlerFunc, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/characters/coins", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	handler(rec, r)
	return rec
}
func coinsAnswer(cp, sp, ep, gp, pp int64) roomAnswer {
	return roomAnswer{
		columns: []string{"copper", "silver", "electrum", "gold", "platinum"},
		values:  []driver.Value{cp, sp, ep, gp, pp},
	}
}
func coinApp(answers ...roomAnswer) (*App, *roomDB) {
	stub := &roomDB{rows: 1, answers: answers}
	db := stub.db()
	return &App{DB: db, Queries: queries.New(db), Hub: nopHub{}}, stub
}
func TestSaveCoinsWritesOnlyTheCoinColumnsClamped(t *testing.T) {
	app, db := newPanelApp(1)
	rec := coinRequest(t, app.SaveCharacterCoins, url.Values{
		"copper":   {"3"},
		"silver":   {"-2"},
		"electrum": {""},
		"gold":     {"97"},
		"platinum": {"99999999999"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	call := db.only(t)
	if got, want := sortedColumns(t, call.query), []string{"copper", "electrum", "gold", "platinum", "silver"}; !reflect.DeepEqual(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
	if got, want := call.args[:5], []any{uint32(3), uint32(0), uint32(0), uint32(97), uint32(dnd.CoinLimit)}; !reflect.DeepEqual(got, want) {
		t.Errorf("coins = %v, want %v", got, want)
	}
	for i, want := range []any{testCharacterID, testOwnerID} {
		if got := call.args[5+i]; got != want {
			t.Errorf("scope[%d] = %v, want %v", i, got, want)
		}
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="errors-coins-` + testCharacterID.String() + `"`,
		`id="coin-total-` + testCharacterID.String() + `"`,
		`hx-swap-oob="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reply is missing %s\n%s", want, body)
		}
	}
}
func TestCoinChangeRefusesWhatItCannotReadWithoutTouchingTheDatabase(t *testing.T) {
	app, db := newPanelApp(1)
	rec := coinRequest(t, app.ChangeCharacterCoins, url.Values{"change": {" -25 "}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(db.calls)+len(db.reads) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls)+len(db.reads))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="coin-change-` + testCharacterID.String() + `"`,
		`value="-25"`,
		`aria-invalid="true"`,
		"like +10gp or -25sp",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("refusal is missing %s\n%s", want, body)
		}
	}
}
func TestCoinChangeMakesChangeInsideOneTransaction(t *testing.T) {
	app, db := coinApp(coinsAnswer(0, 0, 0, 100, 0))
	rec := coinRequest(t, app.ChangeCharacterCoins, url.Values{"change": {"-25sp"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d\n%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	steps := db.recordedSteps()
	if len(steps) != 4 || steps[0] != "BEGIN" || !strings.Contains(steps[1], "FOR UPDATE") ||
		!strings.Contains(steps[2], "UPDATE characters") || steps[3] != "COMMIT" {
		t.Fatalf("steps = %q, want BEGIN, a locking read, the update, COMMIT", steps)
	}
	update := db.recorded()[1]
	if got, want := update.args[:5], []any{int64(0), int64(5), int64(0), int64(97), int64(0)}; !reflect.DeepEqual(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Coins saved.") {
		t.Errorf("no toast: %q", rec.Header().Get("HX-Trigger"))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="coin-change-` + testCharacterID.String() + `"`,
		`id="coin-counts-` + testCharacterID.String() + `" class="grid gap-2"`,
		`name="silver" type="number"`,
		`value="97"`,
		`value="5"`,
		"97.5 gp",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reply is missing %s\n%s", want, body)
		}
	}
	if strings.Contains(body, `value="-25sp"`) {
		t.Error("the change box kept what was typed after it was applied")
	}
	if got := strings.Count(body, `hx-swap-oob="true"`); got != 2 {
		t.Errorf("out-of-band swaps = %d, want the counts and the total", got)
	}
}
func TestCoinChangeTooDearRollsBackAndSaysWhatThePurseHolds(t *testing.T) {
	app, db := coinApp(coinsAnswer(5, 0, 0, 1, 0))
	rec := coinRequest(t, app.ChangeCharacterCoins, url.Values{"change": {"-2gp"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	steps := db.recordedSteps()
	if steps[len(steps)-1] != "ROLLBACK" {
		t.Errorf("steps = %q, want the transaction rolled back", steps)
	}
	for _, step := range steps {
		if strings.Contains(step, "UPDATE characters") {
			t.Errorf("wrote coins it could not afford:\n%s", step)
		}
	}
	body := rec.Body.String()
	for _, want := range []string{`value="-2gp"`, "holds 1.05 gp in all"} {
		if !strings.Contains(body, want) {
			t.Errorf("refusal is missing %s\n%s", want, body)
		}
	}
}
func TestCoinChangeOnAMissingCharacterIsNotFound(t *testing.T) {
	app, _ := coinApp()
	rec := coinRequest(t, app.ChangeCharacterCoins, url.Values{"change": {"+1gp"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want it empty", rec.Body.String())
	}
}
