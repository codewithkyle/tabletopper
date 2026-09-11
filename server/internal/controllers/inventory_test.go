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
	scope := call.args[len(call.args)-3:]
	for i, want := range []ulid.ULID{testItemID, testCharacterID, testOwnerID} {
		if got, ok := scope[i].(ulid.ULID); !ok || got != want {
			t.Errorf("scope[%d] = %v, want %v", i, scope[i], want)
		}
	}
}
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
func TestAddInventoryItemCannotCarryItemData(t *testing.T) {
	app, db := newPanelApp(0)
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
		if !strings.Contains(body, "character_id") && !strings.Contains(body, "characters") {
			t.Errorf("%s is not scoped to the character:\n%s", name, body)
		}
	}
}
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
