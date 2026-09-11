package controllers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var testAssetID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS2")










func TestUploadJournalImageRejectsUnparseableIDs(t *testing.T) {
	for _, c := range []struct{ name, character, entry string }{
		{name: "character", character: "not-a-ulid", entry: testEntryID.String()},
		{name: "entry", character: testCharacterID.String(), entry: "not-a-ulid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			r := httptest.NewRequest(http.MethodPost, "/characters/x/journal/y/images", nil)
			r.SetPathValue("id", c.character)
			r.SetPathValue("entryId", c.entry)
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

			rec := httptest.NewRecorder()
			app.UploadJournalImage(rec, r)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if !strings.Contains(rec.Header().Get("HX-Trigger"), "alert") {
				t.Errorf("no alert in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}



func TestGetJournalImageRejectsUnparseableIDs(t *testing.T) {
	for _, c := range []struct{ name, character, entry, asset string }{
		{name: "character", character: "not-a-ulid", entry: testEntryID.String(), asset: testAssetID.String()},
		{name: "entry", character: testCharacterID.String(), entry: "not-a-ulid", asset: testAssetID.String()},
		{name: "asset", character: testCharacterID.String(), entry: testEntryID.String(), asset: "not-a-ulid"},
	} {
		t.Run(c.name, func(t *testing.T) {
			app, db := newPanelApp(1)

			r := httptest.NewRequest(http.MethodGet, "/characters/x/journal/y/images/z", nil)
			r.SetPathValue("id", c.character)
			r.SetPathValue("entryId", c.entry)
			r.SetPathValue("assetId", c.asset)
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

			rec := httptest.NewRecorder()
			app.GetJournalImage(rec, r)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if got := rec.Header().Get("HX-Trigger"); got != "" {
				t.Errorf("HX-Trigger = %q, want none", got)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}





func TestJournalImagePath(t *testing.T) {
	want := "/characters/" + testCharacterID.String() +
		"/journal/" + testEntryID.String() +
		"/images/" + testAssetID.String()

	if got := journalImagePath(testCharacterID, testEntryID, testAssetID); got != want {
		t.Errorf("journalImagePath = %q, want %q", got, want)
	}
}




func TestJournalImageFlips(t *testing.T) {
	detached := sql.NullTime{Time: time.Now(), Valid: true}

	referenced := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS3")
	unreferenced := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS4")

	for _, c := range []struct {
		name           string
		states         []queries.ListJournalImageStatesRow
		attach, detach []ulid.ULID
	}{
		{
			name:   "referenced and detached is attached",
			states: []queries.ListJournalImageStatesRow{{ID: referenced, DetachedAt: detached}},
			attach: []ulid.ULID{referenced},
		},
		{
			name:   "unreferenced and attached is detached",
			states: []queries.ListJournalImageStatesRow{{ID: unreferenced}},
			detach: []ulid.ULID{unreferenced},
		},
		{
			name:   "referenced and attached is left alone",
			states: []queries.ListJournalImageStatesRow{{ID: referenced}},
		},
		{
			name:   "unreferenced and detached is left alone",
			states: []queries.ListJournalImageStatesRow{{ID: unreferenced, DetachedAt: detached}},
		},
		{
			name: "both at once, in one pass",
			states: []queries.ListJournalImageStatesRow{
				{ID: referenced, DetachedAt: detached},
				{ID: unreferenced},
			},
			attach: []ulid.ULID{referenced},
			detach: []ulid.ULID{unreferenced},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			attach, detach := journalImageFlips(c.states, func(id ulid.ULID) bool {
				return id == referenced
			})

			if !sameIDs(attach, c.attach) {
				t.Errorf("attach = %v, want %v", attach, c.attach)
			}
			if !sameIDs(detach, c.detach) {
				t.Errorf("detach = %v, want %v", detach, c.detach)
			}
		})
	}
}







func TestJournalImageFlipsCountsAReferenceInACodeFence(t *testing.T) {
	body := "Here is how the URL is built:\n\n```\n" +
		journalImagePath(testCharacterID, testEntryID, testAssetID) +
		"\n```\n"

	states := []queries.ListJournalImageStatesRow{
		{ID: testAssetID, DetachedAt: sql.NullTime{Time: time.Now(), Valid: true}},
	}
	attach, detach := journalImageFlips(states, func(id ulid.ULID) bool {
		return strings.Contains(body, journalImagePath(testCharacterID, testEntryID, id))
	})

	if !sameIDs(attach, []ulid.ULID{testAssetID}) {
		t.Errorf("attach = %v, want the fenced image", attach)
	}
	if len(detach) != 0 {
		t.Errorf("detach = %v, want none", detach)
	}
}

func sameIDs(got, want []ulid.ULID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}






func TestSaveReconcilesAfterTheUpdate(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())

	if len(db.calls) != 2 {
		t.Fatalf("statements run = %d, want 2", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "UPDATE journals") {
		t.Fatalf("the first statement is not the save:\n%s", db.calls[0].query)
	}

	read := db.calls[1]
	if !strings.Contains(read.query, "FROM assets") || !strings.Contains(read.query, "journal_id") {
		t.Fatalf("the second statement is not the image read:\n%s", read.query)
	}
	
	
	if entry, ok := read.args[0].(*ulid.ULID); !ok || entry == nil || *entry != testEntryID {
		t.Errorf("read scoped to entry %v, want %v", read.args[0], testEntryID)
	}
	if owner, ok := read.args[1].(ulid.ULID); !ok || owner != testOwnerID {
		t.Errorf("read scoped to owner %v, want %v", read.args[1], testOwnerID)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-journal" hidden></div>` {
		t.Errorf("body = %q, want the cleared error block", body)
	}
}






func TestSaveThatMatchedNothingDoesNotReconcile(t *testing.T) {
	app, db := newPanelApp(0)

	rec := journalRequest(t, app.SaveJournalEntry, http.MethodPost, journalForm(), testEntryID.String())

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "UPDATE journals") {
		t.Errorf("ran something other than the save:\n%s", db.calls[0].query)
	}
}








func TestDeleteJournalEntryRevokesAndDetachesBeforeDeleting(t *testing.T) {
	app, db := newPanelApp(1)

	journalRequest(t, app.DeleteJournalEntry, http.MethodDelete, nil, testEntryID.String())

	if len(db.calls) != 3 {
		t.Fatalf("statements run = %d, want 3", len(db.calls))
	}

	revoke := db.calls[0]
	if !strings.Contains(revoke.query, "DELETE FROM shares") {
		t.Fatalf("the first statement is not the revoke:\n%s", revoke.query)
	}
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := boundID(revoke.args[i]); !ok || got != want {
			t.Errorf("revoke arg %d = %v, want %v", i, revoke.args[i], want)
		}
	}

	detach := db.calls[1]
	if !strings.Contains(detach.query, "UPDATE assets") || !strings.Contains(detach.query, "detached_at") {
		t.Fatalf("the second statement is not the detach:\n%s", detach.query)
	}
	if entry, ok := detach.args[0].(*ulid.ULID); !ok || entry == nil || *entry != testEntryID {
		t.Errorf("detach scoped to entry %v, want %v", detach.args[0], testEntryID)
	}
	if owner, ok := detach.args[1].(ulid.ULID); !ok || owner != testOwnerID {
		t.Errorf("detach scoped to owner %v, want %v", detach.args[1], testOwnerID)
	}

	del := db.calls[2]
	if !strings.Contains(del.query, "DELETE FROM journals") {
		t.Fatalf("the third statement is not the delete:\n%s", del.query)
	}
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := del.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("delete arg %d = %v, want %v", i, del.args[i], want)
		}
	}
}
