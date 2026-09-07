package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
)

// searchRequest is one call at the shared search route, signed in as the test
// owner. Every case here is about what the handler sends to the database, so
// the reply is always a failed read -- recordingDB has no rows to give -- and
// the statement it recorded on the way is the thing under test.
func searchRequest(t *testing.T, query string) (*recordingDB, *deadlineRecorder) {
	t.Helper()

	db := &recordingDB{err: errNoRowsToGive}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodGet, "/fragment/assets/list?"+query, nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := newRecorder()
	app.AssetListFragment(rec, r)

	return db, rec
}

// THE KIND IS THE ONE VALUE IN THIS MANAGER THAT COMES OFF THE WIRE, and it is
// matched against the four members before a statement runs. Everywhere else the
// kind is a path segment bound at registration, so nothing else here has ever
// had to check one.
//
// A kind that is not one of the four is an empty 404 and NOT an alert: the box
// carries its kind in its own hx-get, so a bad one cannot come from the page and
// there is nobody on the other end to tell.
func TestTheAssetSearchAnswersOnlyTheFourKinds(t *testing.T) {
	for name, c := range map[string]struct {
		kind  string
		table string
	}{
		"maps":    {"maps", "type = 'map'"},
		"tokens":  {"tokens", "type = ?"},
		"avatars": {"avatars", "type = ?"},
		"music":   {"music", "type = 'music'"},
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := searchRequest(t, "kind="+c.kind+"&q=goblin")

			if len(db.calls) != 1 {
				t.Fatalf("ran %d statements, want 1", len(db.calls))
			}
			if !strings.Contains(db.calls[0].query, c.table) {
				t.Errorf("the search is not scoped to %s: %q", name, db.calls[0].query)
			}
			if !strings.Contains(db.calls[0].query, "name LIKE ?") {
				t.Errorf("the search does not filter by name: %q", db.calls[0].query)
			}
		})
	}

	// "images" is the sibling segment of the four pages and is the one that
	// would hurt: /assets/images/{id} serves any signed-in user any picture by
	// id, and a kind that reached a statement would be a listing of them.
	for _, kind := range []string{"", "images", "monsters", "journal", "character", "Maps", "maps%20"} {
		t.Run("refuses "+kind, func(t *testing.T) {
			db, rec := searchRequest(t, "kind="+kind+"&q=goblin")

			if len(db.calls) != 0 {
				t.Errorf("%q reached the database: %q", kind, db.calls[0].query)
			}
			if rec.Code != http.StatusNotFound {
				t.Errorf("%q answered %d, want 404", kind, rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("%q answered with a body: %q", kind, rec.Body.String())
			}
		})
	}
}

// A term longer than the column is refused before anything is queried, for the
// reason the monster search gives: the box carries a maxlength, so a term past
// the column's width came from something other than the box.
func TestAnOverlongSearchTermIsRefusedWithoutQuerying(t *testing.T) {
	db, rec := searchRequest(t, "kind=maps&q="+strings.Repeat("a", pages.AssetNameLimit+1))

	if len(db.calls) != 0 {
		t.Errorf("an overlong term reached the database: %q", db.calls[0].query)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("answered %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("answered with a body: %q", rec.Body.String())
	}

	// Counted in characters and not bytes, the way the column and maxlength
	// both count. A limit measured in bytes would refuse a name of 90 accented
	// characters that MySQL would have stored without complaint.
	db, rec = searchRequest(t, "kind=maps&q="+strings.Repeat("é", pages.AssetNameLimit))
	if len(db.calls) != 1 {
		t.Errorf("a term of %d characters was refused as too long", pages.AssetNameLimit)
	}
	if rec.Code == http.StatusNotFound {
		t.Error("a term the column holds was answered 404")
	}
}

// WHAT LIKE READS AS A PATTERN IS NOT WHAT SOMEBODY TYPING READS AS ONE. `%`
// and `_` are wildcards to MySQL and ordinary characters to a person, so an
// unescaped `%` in the box would match the whole shelf and `_` would quietly
// match any character at all.
func TestASearchTermIsEscapedBeforeItReachesLike(t *testing.T) {
	db, _ := searchRequest(t, "kind=tokens&q=50%25_off")

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}

	want := `%50\%\_off%`
	var found bool
	for _, arg := range db.calls[0].args {
		if s, ok := arg.(string); ok && s == want {
			found = true
		}
	}
	if !found {
		t.Errorf("the term was not escaped into %q: %v", want, db.calls[0].args)
	}
}

// AN EMPTY BOX IS THE WHOLE SHELF AND NOT A SEARCH FOR NOTHING, which is what
// the reader means by clearing it. It is trimmed first, so a box holding a
// space somebody is still typing around is the whole shelf too rather than a
// search for a space.
func TestAnEmptySearchIsTheWholeShelf(t *testing.T) {
	for _, q := range []string{"", "q=", "q=%20%20"} {
		db, _ := searchRequest(t, "kind=avatars&"+q)

		if len(db.calls) != 1 {
			t.Fatalf("%q ran %d statements, want 1", q, len(db.calls))
		}
		if strings.Contains(db.calls[0].query, "LIKE") {
			t.Errorf("%q ran the search statement rather than the listing: %q", q, db.calls[0].query)
		}
	}
}

// The music search carries uploaded_at IS NOT NULL over from the listing it is
// a copy of. Without it, typing a letter of an abandoned upload's name would put
// a card on the page for a track that is not in the bucket -- a player that
// answers every press with a 404.
func TestTheMusicSearchStillDropsUnfinishedUploads(t *testing.T) {
	db, _ := searchRequest(t, "kind=music&q=rain")

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "uploaded_at IS NOT NULL") {
		t.Errorf("the music search lists half-finished uploads: %q", db.calls[0].query)
	}
}

// Every search is scoped to the account doing it. There is no id in this URL, so
// this is the only thing standing between a search box and somebody else's
// shelf.
func TestEveryAssetSearchIsScopedToTheSessionsOwner(t *testing.T) {
	for _, kind := range []string{"maps", "tokens", "avatars", "music"} {
		db, _ := searchRequest(t, "kind="+kind+"&q=goblin")

		if len(db.calls) != 1 {
			t.Fatalf("%s ran %d statements, want 1", kind, len(db.calls))
		}
		if !strings.Contains(db.calls[0].query, "owner_id = ?") {
			t.Errorf("the %s search is not scoped to an owner: %q", kind, db.calls[0].query)
		}

		var owner bool
		for _, arg := range db.calls[0].args {
			if id, ok := boundID(arg); ok && id == testOwnerID {
				owner = true
			}
		}
		if !owner {
			t.Errorf("the %s search is not scoped to the session's user: %v", kind, db.calls[0].args)
		}
	}
}
