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

var testItemID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS0")

// inventoryRequest is panelPost with a method, because two of these routes are
// not POSTs. Everything else is the same: the path values the route declares,
// and a session to be scoped by.
func inventoryRequest(t *testing.T, handler http.HandlerFunc, method string, form url.Values, itemID string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, "/characters/inventory", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	if itemID != "" {
		r.SetPathValue("itemId", itemID)
	}
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

func fullInventoryForm() url.Values {
	return url.Values{
		"name":        {"Longsword"},
		"quantity":    {"2"},
		"value":       {"15 gp"},
		"weight":      {"3"},
		"equipped":    {"1"},
		"description": {"1d8 slashing, versatile (1d10)"},
	}
}

// A row save writes its own row and nothing else. The character's own columns
// are the thing it must not be able to reach: the sheet's parse helpers return
// their fallback on an empty string, so a handler wide enough to touch them
// would write 10 over every ability score and report success.
func TestSaveInventoryItemWritesOnlyItsOwnColumns(t *testing.T) {
	app, db := newPanelApp(1)

	rec := inventoryRequest(t, app.SaveInventoryItem, http.MethodPost, fullInventoryForm(), testItemID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "UPDATE inventory") {
		t.Fatalf("did not update inventory:\n%s", call.query)
	}

	want := []string{"description", "equipped", "name", "quantity", "value", "weight"}
	if got := sortedColumns(t, call.query); !reflect.DeepEqual(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}

	// All three of the scoping values, not just the owner. The item id and the
	// character id both arrive in the URL, so the statement has to carry both or
	// an item could be written through a character it does not belong to.
	scope := call.args[len(call.args)-3:]
	for i, want := range []ulid.ULID{testItemID, testCharacterID, testOwnerID} {
		if got, ok := scope[i].(ulid.ULID); !ok || got != want {
			t.Errorf("scope[%d] = %v, want %v", i, scope[i], want)
		}
	}
}

// THE CHECKBOX TEST. `equipped` is read from whether the field arrived at all,
// because an unchecked box posts nothing -- which is correct for a checkbox and
// is also the exact shape the panel handlers exist to avoid. What makes it safe
// is that the row form always renders all six controls together, so a post
// without `equipped` really is an unticked box and not half a form. That is a
// property of the markup, so it is pinned here and in the page tests rather than
// left to be rediscovered.
func TestEquippedIsReadFromTheAbsenceOfTheField(t *testing.T) {
	for _, c := range []struct {
		name string
		form url.Values
		want bool
	}{
		{name: "ticked", form: fullInventoryForm(), want: true},
		{name: "unticked", form: func() url.Values {
			f := fullInventoryForm()
			f.Del("equipped")
			return f
		}(), want: false},
		{name: "empty form", form: url.Values{}, want: false},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			inventoryRequest(t, app.SaveInventoryItem, http.MethodPost, c.form, testItemID.String())

			call := db.only(t)
			// Fifth of the six SET values, in the order the generated statement
			// binds them: name, quantity, value, weight, equipped, description.
			got, ok := call.args[4].(bool)
			if !ok {
				t.Fatalf("equipped arg is %T, want bool", call.args[4])
			}
			if got != c.want {
				t.Errorf("equipped = %v, want %v", got, c.want)
			}
		})
	}
}

// Quantity and weight are coerced rather than rejected. type=number cannot
// produce most of these, so a rejection would be a message nobody could have
// caused; the ones a person can cause -- an empty field mid-retype -- have to
// mean something sensible instead of failing the save.
func TestInventoryNumbersAreCoercedNotRejected(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want uint32
	}{
		{raw: "", want: 1},
		{raw: "   ", want: 1},
		{raw: "not a number", want: 1},
		{raw: "0", want: 0},
		{raw: "-4", want: 0},
		{raw: "7", want: 7},
		{raw: "99999999999999", want: inventoryQuantityLimit},
	} {
		if got := parseInventoryQuantity(c.raw); got != c.want {
			t.Errorf("quantity %q = %d, want %d", c.raw, got, c.want)
		}
	}

	for _, c := range []struct {
		raw  string
		want float64
	}{
		{raw: "", want: 0},
		{raw: "0", want: 0},
		{raw: "-2.5", want: 0},
		{raw: "0.05", want: 0.05},
		{raw: "3", want: 3},
		// ParseFloat takes both of these WITHOUT an error, and DECIMAL takes
		// neither. NaN fails every comparison, so it needs its own branch.
		{raw: "NaN", want: 0},
		{raw: "Inf", want: inventoryWeightLimit},
		{raw: "-Inf", want: 0},
		{raw: "99999999", want: inventoryWeightLimit},
	} {
		if got := parseInventoryWeight(c.raw); got != c.want {
			t.Errorf("weight %q = %v, want %v", c.raw, got, c.want)
		}
	}
}

// An unweighed item and a weightless one are the same row, and neither should
// put a "0" in the field. Trailing zeros come off too: the column stores 3.00.
func TestWeightRendersBlankAtZero(t *testing.T) {
	for _, c := range []struct {
		weight float64
		want   string
	}{
		{weight: 0, want: ""},
		{weight: 3, want: "3"},
		{weight: 0.5, want: "0.5"},
		{weight: 0.05, want: "0.05"},
	} {
		if got := formatInventoryWeight(c.weight); got != c.want {
			t.Errorf("weight %v renders %q, want %q", c.weight, got, c.want)
		}
	}
}

// The add takes no form at all, and the statement is what enforces that: it
// selects from characters, so there is nowhere for item data to enter and no way
// to hang a row off a character the sender does not own.
func TestAddInventoryItemCannotCarryItemData(t *testing.T) {
	app, db := newPanelApp(0)

	// rows=0 stands for "that character is not yours", which is the only thing
	// zero can mean here -- the id is freshly minted, so a duplicate key is not
	// on the table. It also stops the handler before the read-back, which this
	// fake cannot serve.
	rec := inventoryRequest(t, app.AddInventoryItem, http.MethodPost, fullInventoryForm(), "")
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

	if fields := reflect.TypeOf(queries.InsertInventoryItemParams{}).NumField(); fields != 3 {
		t.Errorf("InsertInventoryItemParams has %d fields, want 3 (item, character, owner)", fields)
	}
}

// THE DELETE MUST BE A 200. base.templ configures noSwap for 204, and a status
// in that list sets the swap to "none" -- which overrides the hx-swap="delete"
// on the button and leaves the row on screen after the database has dropped it.
// Nothing about that failure is visible from the server side, so it is pinned
// here.
func TestDeleteInventoryItemAnswers200SoTheRowIsSwappedOut(t *testing.T) {
	app, db := newPanelApp(1)

	rec := inventoryRequest(t, app.DeleteInventoryItem, http.MethodDelete, nil, testItemID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d -- 204 is in noSwap and would strand the row", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "DELETE FROM inventory") {
		t.Fatalf("did not delete from inventory:\n%s", call.query)
	}
	for i, want := range []ulid.ULID{testItemID, testCharacterID, testOwnerID} {
		if got, ok := call.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("arg[%d] = %v, want %v", i, call.args[i], want)
		}
	}
}

// A row that is already gone -- deleted in another tab -- is a 404 about the
// item. finishPanel says "character" for the same condition, which is why this
// path does not go through it: sending someone to look for a missing character
// when their character is fine wastes the one thing the message is for.
func TestMissingInventoryRowIsAnItem404(t *testing.T) {
	for _, c := range []struct {
		name    string
		handler func(*App) http.HandlerFunc
		method  string
	}{
		{name: "save", handler: func(a *App) http.HandlerFunc { return a.SaveInventoryItem }, method: http.MethodPost},
		{name: "delete", handler: func(a *App) http.HandlerFunc { return a.DeleteInventoryItem }, method: http.MethodDelete},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, _ := newPanelApp(0)

			rec := inventoryRequest(t, c.handler(app), c.method, fullInventoryForm(), testItemID.String())
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "item") {
				t.Errorf("the alert does not name the item: %s", trigger)
			}
		})
	}
}

// An item id that is not a ULID never reaches a query. The path segment lands in
// three attributes and a URL on the way back out, so it is parsed before
// anything is rendered or run.
func TestUnparseableItemIDTouchesNoDatabase(t *testing.T) {
	app, db := newPanelApp(1)

	rec := inventoryRequest(t, app.SaveInventoryItem, http.MethodPost, fullInventoryForm(), "not-a-ulid")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
}

// The column widths are enforced here because MySQL runs in strict mode: without
// this the driver's error would reach the user as a 500 on a field they were
// entitled to overfill by pasting.
func TestOverlongInventoryFieldsAreRejectedNotTruncated(t *testing.T) {
	for _, c := range []struct {
		name  string
		field string
		value string
	}{
		{name: "name", field: "name", value: strings.Repeat("é", inventoryNameLimit+1)},
		{name: "value", field: "value", value: strings.Repeat("é", inventoryValueLimit+1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			form := fullInventoryForm()
			form.Set(c.field, c.value)

			rec := inventoryRequest(t, app.SaveInventoryItem, http.MethodPost, form, testItemID.String())
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}

	// Measured in characters, not bytes, because that is what varchar counts. A
	// name of exactly the limit in accented letters is three times the limit in
	// bytes and the column takes it without complaint.
	app, db := newPanelApp(1)
	form := fullInventoryForm()
	form.Set("name", strings.Repeat("é", inventoryNameLimit))

	rec := inventoryRequest(t, app.SaveInventoryItem, http.MethodPost, form, testItemID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1", len(db.calls))
	}
}

// EVERY inventory query is scoped to the owner, reads included. The writes are
// covered above by watching what reaches the driver, but a SELECT that dropped
// its owner_id would leak another user's items onto a page and nothing in the
// handler would notice -- the rows would arrive and render. So the statements
// themselves are checked, which is also the only way to reach the two reads:
// they go through QueryContext, and the fake pool the write tests use cannot
// serve one.
func TestEveryInventoryQueryIsScopedToTheOwner(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "inventory.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}

	statements := regexp.MustCompile(`(?m)^-- name: (\w+)`).FindAllStringSubmatchIndex(string(source), -1)
	if len(statements) == 0 {
		t.Fatal("no named queries in sql/inventory.sql")
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
		// The insert names the character in its own WHERE, so it is covered by
		// the same rule; everything else has to name it too, or an item could be
		// reached through a character it does not belong to.
		if !strings.Contains(body, "character_id") && !strings.Contains(body, "characters") {
			t.Errorf("%s is not scoped to the character:\n%s", name, body)
		}
	}
}

// Only the ticked rows reach the Character page. It shows three of forty and has
// no use for the rest, and a filter applied in Go instead would mean loading a
// whole inventory to render a corner of one page.
func TestEquippedQueryFiltersInSQL(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "sql", "inventory.sql"))
	if err != nil {
		t.Fatalf("cannot read the queries: %v", err)
	}

	body := string(source)
	start := strings.Index(body, "-- name: ListEquippedInventory")
	if start < 0 {
		t.Fatal("no ListEquippedInventory query")
	}
	end := strings.Index(body[start+1:], "-- name:")
	statement := body[start:]
	if end >= 0 {
		statement = body[start : start+1+end]
	}

	if !strings.Contains(statement, "equipped = TRUE") {
		t.Errorf("the equipped view is not filtered in the statement:\n%s", statement)
	}
}
