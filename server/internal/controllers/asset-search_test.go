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
	db, rec = searchRequest(t, "kind=maps&q="+strings.Repeat("é", pages.AssetNameLimit))
	if len(db.calls) != 1 {
		t.Errorf("a term of %d characters was refused as too long", pages.AssetNameLimit)
	}
	if rec.Code == http.StatusNotFound {
		t.Error("a term the column holds was answered 404")
	}
}
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
func TestTheMusicSearchStillDropsUnfinishedUploads(t *testing.T) {
	db, _ := searchRequest(t, "kind=music&q=rain")
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "uploaded_at IS NOT NULL") {
		t.Errorf("the music search lists half-finished uploads: %q", db.calls[0].query)
	}
}
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
