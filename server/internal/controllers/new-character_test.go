package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
)

// Creation is one field and one statement. These pin both halves: that a name
// is all the handler will take, and that a name is all the statement can carry.

// The happy path. The reply is a redirect and no body, because the dialog the
// post came from is about to be navigated away from.
func TestCreateFromNameRedirectsToTheEditor(t *testing.T) {
	app, db := newPanelApp(1)

	rec := panelPost(t, db, app.NewCharacterForm, url.Values{"name": {"  Ferdinand the Bold  "}}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "INSERT INTO characters") {
		t.Errorf("statement is not the create: %q", db.calls[0].query)
	}

	// The name is trimmed before it is stored and before it is announced.
	name, ok := db.calls[0].args[2].(string)
	if !ok || name != "Ferdinand the Bold" {
		t.Errorf("stored name = %v, want %q", db.calls[0].args[2], "Ferdinand the Bold")
	}

	// The owner comes from the session, never from the form.
	if owner := db.calls[0].args[1]; owner != testOwnerID {
		t.Errorf("owner = %v, want %v", owner, testOwnerID)
	}

	id, ok := db.calls[0].args[0].(interface{ String() string })
	if !ok {
		t.Fatalf("first argument is not an id: %T", db.calls[0].args[0])
	}
	want := "/characters/" + id.String() + "/edit"
	if got := rec.Header().Get("HX-Redirect"); got != want {
		t.Errorf("HX-Redirect = %q, want %q", got, want)
	}

	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want empty", body)
	}

	// The toast rides along on the same response; toast.js parks it for the
	// page being navigated to rather than showing it on the way out.
	var events map[string]any
	if err := json.Unmarshal([]byte(rec.Header().Get("HX-Trigger")), &events); err != nil {
		t.Fatalf("HX-Trigger is not JSON: %v", err)
	}
	if events["flash:toast"] != "Ferdinand the Bold has been created." {
		t.Errorf("toast = %v", events["flash:toast"])
	}
}

// A rejection has to be a 422 specifically. It is the only 4xx the dialog's form
// carries an hx-status route for -- every other code in the range is in the
// noSwap list in base.templ, so the reply would land nowhere and the dialog
// would look like it had done nothing.
func TestCreateRejectsBadNamesWithoutWriting(t *testing.T) {
	for _, c := range []struct {
		name string
		//nolint:revive // the field is the posted value, not a description
		value string
		want  string
	}{
		{name: "empty", value: "", want: "Name is required."},
		{name: "whitespace only", value: "   \t ", want: "Name is required."},
		{name: "too long", value: strings.Repeat("a", 129), want: "Name must be 128 characters or fewer."},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := panelPost(t, db, app.NewCharacterForm, url.Values{"name": {c.value}}, nil)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
			if rec.Header().Get("HX-Redirect") != "" {
				t.Error("rejected create still sent HX-Redirect")
			}

			body := rec.Body.String()
			if !strings.Contains(body, c.want) {
				t.Errorf("body missing %q: %s", c.want, body)
			}
			// Into the block the form targets, not the form itself -- so the
			// name the user typed is still in the field.
			if !strings.Contains(body, `id="errors-new-character"`) {
				t.Errorf("body is not the error block: %s", body)
			}
		})
	}
}

// The column is varchar(128) and MySQL counts characters there. A byte-length
// check would reject this name at 128 letters the database would have taken.
func TestCreateMeasuresTheNameInCharactersNotBytes(t *testing.T) {
	app, db := newPanelApp(1)

	name := strings.Repeat("é", 128)
	rec := panelPost(t, db, app.NewCharacterForm, url.Values{"name": {name}}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
}

// THE POINT OF THE REWORK. Creation used to read all 31 editable columns through
// one builder, and the parse helpers it ran on answer an absent field with a
// default rather than an error -- the same property that makes a wide statement
// dangerous on a panel save. The statement is three values wide now, so a post
// carrying a sheet cannot write one whatever the handler does with it.
func TestCreateCannotCarrySheetData(t *testing.T) {
	app, db := newPanelApp(1)

	sheet := url.Values{
		"name":           {"Ferdinand"},
		"str":            {"18"},
		"dex":            {"18"},
		"max_hp":         {"99"},
		"xp":             {"355000"},
		"speed":          {"60 ft."},
		"languages":      {"Common, Draconic"},
		"features-name":  {"Second Wind"},
		"features-value": {"Once per rest"},
		"spell_save_dc":  {"19"},
	}
	rec := panelPost(t, db, app.NewCharacterForm, sheet, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}

	call := db.calls[0]
	if len(call.args) != 3 {
		t.Errorf("statement took %d values, want 3: %v", len(call.args), call.args)
	}
	if placeholders := strings.Count(call.query, "?"); placeholders != 3 {
		t.Errorf("statement has %d placeholders, want 3:\n%s", placeholders, call.query)
	}
	for _, arg := range call.args {
		if s, ok := arg.(string); ok && s != "Ferdinand" {
			t.Errorf("a form value other than the name reached the database: %q", s)
		}
	}

	// Independent of the SQL text: the generated params struct is the other
	// place a fourth value would have to appear.
	if fields := reflect.TypeOf(queries.CreateCharacterFromNameParams{}).NumField(); fields != 3 {
		t.Errorf("CreateCharacterFromNameParams has %d fields, want 3 (id, owner, name)", fields)
	}
}

// The dialog's content is the same for every user, so the handler that serves it
// should not be reaching for a row to render it.
func TestNewCharacterFragmentTouchesNoDatabase(t *testing.T) {
	app, db := newPanelApp(1)

	r := httptest.NewRequest(http.MethodGet, "/fragment/character/new", nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.NewCharacterFragment(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 0 {
		t.Errorf("ran %d statements, want 0", len(db.calls))
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}
