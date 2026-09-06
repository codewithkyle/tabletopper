package controllers

import (
	"context"
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

// THE OWNERSHIP CHECK IS OUT OF REACH IN HERE, and it is worth saying why once
// rather than in each test below. CountJournalImages is a :one, which
// recordingDB answers by panicking -- a *sql.Row cannot be built outside
// database/sql -- so nothing past it can be driven from a handler test. What is
// reachable is everything before it, which is exactly the part these two check:
// an id that does not parse never becomes a query.

// Neither id reaches a statement, and both answer through htmx.NotFound because
// the caller is the editor's fetch, which reads the alert out of the header.
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

// The serve route answers a plain 404 with no header, because its caller is an
// <img> and there is nothing on the page to swap or to alert.
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

// The exact shape, because two things depend on it being this and nothing else:
// the mux pattern the serve route is registered under, and the substring search
// a save uses to decide an image is still in the body. A change here that the
// route did not follow would detach every image on the next debounce.
func TestJournalImagePath(t *testing.T) {
	want := "/characters/" + testCharacterID.String() +
		"/journal/" + testEntryID.String() +
		"/images/" + testAssetID.String()

	if got := journalImagePath(testCharacterID, testEntryID, testAssetID); got != want {
		t.Errorf("journalImagePath = %q, want %q", got, want)
	}
}

// The four state pairs, and only two of them are writes. The other two are what
// makes a steady save cost nothing: a writer typing prose around pictures that
// are already there flips nothing at all.
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

// A URL INSIDE A CODE FENCE KEEPS ITS IMAGE, and that is intended. The
// reference test is a substring search rather than a markdown parse, which is
// exact in the direction that matters -- an image cannot render without its URL
// in the body, so nothing in use is ever detached -- and over-counts in the
// other. The cost is one object for as long as somebody leaves the URL written
// out in prose, which is not worth a parser in the save path.
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

// THE RECONCILIATION RUNS AFTER THE WRITE, NOT INSTEAD OF PART OF IT. The save
// is answered on the strength of the UPDATE alone, and the image bookkeeping
// that follows is best effort: here the read fails -- recordingDB cannot hand
// back rows -- and the reply is still the 200 and the cleared error block the
// writer's next keystroke depends on.
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
	// Scoped by the entry and the owner. The entry is a pointer because
	// journal_id is nullable for every other asset type.
	if entry, ok := read.args[0].(*ulid.ULID); !ok || entry == nil || *entry != testEntryID {
		t.Errorf("read scoped to entry %v, want %v", read.args[0], testEntryID)
	}
	if owner, ok := read.args[1].(ulid.ULID); !ok || owner != testOwnerID {
		t.Errorf("read scoped to owner %v, want %v", read.args[1], testOwnerID)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `<div id="errors-journal"></div>` {
		t.Errorf("body = %q, want the cleared error block", body)
	}
}

// Zero matched rows is an entry that is gone or was never this user's, and
// there is nothing to reconcile against: the body in hand was never stored. A
// reconciliation there would read a stranger's images -- scoped by the owner,
// so it would find none -- and then detach nothing, which is a round trip to
// learn what the 404 already said.
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

// THE DETACH RUNS FIRST, and the order is the test. It finds the images through
// journal_id, so after the entry row is gone it would match nothing and forty
// objects would sit in the bucket with no row pointing at them and no sweep
// that would ever find them.
func TestDeleteJournalEntryDetachesBeforeDeleting(t *testing.T) {
	app, db := newPanelApp(1)

	journalRequest(t, app.DeleteJournalEntry, http.MethodDelete, nil, testEntryID.String())

	if len(db.calls) != 2 {
		t.Fatalf("statements run = %d, want 2", len(db.calls))
	}

	detach := db.calls[0]
	if !strings.Contains(detach.query, "UPDATE assets") || !strings.Contains(detach.query, "detached_at") {
		t.Fatalf("the first statement is not the detach:\n%s", detach.query)
	}
	if entry, ok := detach.args[0].(*ulid.ULID); !ok || entry == nil || *entry != testEntryID {
		t.Errorf("detach scoped to entry %v, want %v", detach.args[0], testEntryID)
	}
	if owner, ok := detach.args[1].(ulid.ULID); !ok || owner != testOwnerID {
		t.Errorf("detach scoped to owner %v, want %v", detach.args[1], testOwnerID)
	}

	del := db.calls[1]
	if !strings.Contains(del.query, "DELETE FROM journals") {
		t.Fatalf("the second statement is not the delete:\n%s", del.query)
	}
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := del.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("delete arg %d = %v, want %v", i, del.args[i], want)
		}
	}
}

// DELETING A CHARACTER DETACHES ITS JOURNAL IMAGES THROUGH THE journals TABLE,
// and that is why DeleteCharacter runs this before it empties that table. The
// handler's order cannot be driven from here -- it opens with a :one that
// recordingDB answers by panicking -- so what is pinned is the reason for it:
// the statement's subquery. Rewrite it to find images some other way and the
// ordering in DeleteCharacter stops mattering; leave it and reordering the two
// silently orphans every picture in the character's journal.
func TestDetachingACharactersImagesReadsTheJournalsTable(t *testing.T) {
	app, db := newPanelApp(1)

	err := app.Queries.DetachCharacterJournalImages(context.Background(), queries.DetachCharacterJournalImagesParams{
		OwnerID:     testOwnerID,
		CharacterID: testCharacterID,
	})
	if err != nil {
		t.Fatalf("DetachCharacterJournalImages: %v", err)
	}

	call := db.only(t)
	if !strings.Contains(call.query, "UPDATE assets") || !strings.Contains(call.query, "FROM journals") {
		t.Fatalf("the detach does not find its rows through journals:\n%s", call.query)
	}
	// The owner is bound on both sides of the subquery boundary, which is what
	// keeps one user's delete out of another user's images.
	for i, want := range []ulid.ULID{testOwnerID, testCharacterID, testOwnerID} {
		if got, ok := call.args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("arg %d = %v, want %v", i, call.args[i], want)
		}
	}
}
