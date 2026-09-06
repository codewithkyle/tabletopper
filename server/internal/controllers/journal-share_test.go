package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/share"

	"github.com/oklog/ulid/v2"
)

// THE SHARE TESTS THAT CAN RUN HERE.
//
// recordingDB answers a :one by panicking, so the three handlers that open with
// GetShareByToken or GetJournalShare -- the shared page, both image routes and
// the dialog fragment -- cannot be driven from this harness at all. What is
// covered below is everything that is not a read: the validation the create
// runs before it touches the database, the revoke, and the two pure functions
// that decide what a link says and which pictures a shared page will serve.
//
// The last of those is the one worth having. shareImageSource is the only thing
// standing between a journal body and a reader's browser fetching whatever URL
// somebody typed into it.

func shareForm(values map[string]string) url.Values {
	form := url.Values{}
	for name, value := range values {
		form.Set(name, value)
	}

	return form
}

// A rejected create must not have written anything. The password is hashed and
// the token minted after this point, so a form that gets past validation is one
// that is going to be inserted.
func TestARejectedShareFormRunsNoStatements(t *testing.T) {
	cases := map[string]url.Values{
		"expiry with no days":  shareForm(map[string]string{"expiry": "on", "days": ""}),
		"expiry of zero days":  shareForm(map[string]string{"expiry": "on", "days": "0"}),
		"expiry past a year":   shareForm(map[string]string{"expiry": "on", "days": "366"}),
		"days that is a word":  shareForm(map[string]string{"expiry": "on", "days": "soon"}),
		"password that is one": shareForm(map[string]string{"protect": "on", "password": "abc"}),
	}

	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := journalRequest(t, app.CreateJournalShare, http.MethodPost, form, testEntryID.String())

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("a rejected form ran %d statements:\n%s", len(db.calls), db.calls[0].query)
			}
		})
	}
}

// A toggle that is off discards the field beside it. The days box always posts
// a value -- it carries a default so flipping the toggle is a complete answer
// -- so reading it regardless would give every link an expiry nobody asked for.
func TestATogglesFieldIsIgnoredWhileItsToggleIsOff(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		shareForm(map[string]string{"days": "30", "password": "the black spider"}).Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	input, problems := buildShareInput(r)

	if len(problems) != 0 {
		t.Fatalf("a form with both toggles off was rejected: %v", problems)
	}
	if input.Days != 0 {
		t.Errorf("Days = %d, want 0 -- the expiry toggle was off", input.Days)
	}
	if input.Password != "" {
		t.Error("a password was taken from a form whose protect toggle was off")
	}
}

// A password is a secret rather than a name, so a space at either end of it is
// a character the person who chose it typed. Trimming would store something
// they could not then type back in.
func TestASharePasswordIsNotTrimmed(t *testing.T) {
	password := "  a spider  "
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		shareForm(map[string]string{"protect": "on", "password": password}).Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	input, problems := buildShareInput(r)

	if len(problems) != 0 {
		t.Fatalf("rejected: %v", problems)
	}
	if input.Password != password {
		t.Errorf("Password = %q, want %q", input.Password, password)
	}
}

func TestTheExpiryBoundsAreInclusive(t *testing.T) {
	for _, days := range []string{"1", "365"} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
			shareForm(map[string]string{"expiry": "on", "days": days}).Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		if _, problems := buildShareInput(r); len(problems) != 0 {
			t.Errorf("%s days was rejected: %v", days, problems)
		}
	}
}

// Revoking is one statement, and it is scoped by all three ids: an entry id
// out of the URL means nothing on its own, and the owner comes from the session.
func TestRevokingAShareDeletesOneRowScopedToItsOwner(t *testing.T) {
	app, db := newPanelApp(1)

	rec := journalRequest(t, app.RevokeJournalShare, http.MethodDelete, nil, testEntryID.String())

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}
	if !strings.Contains(db.calls[0].query, "DELETE FROM shares") {
		t.Fatalf("did not delete from shares:\n%s", db.calls[0].query)
	}
	for i, want := range []ulid.ULID{testEntryID, testCharacterID, testOwnerID} {
		if got, ok := db.calls[0].args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("revoke arg %d = %v, want %v", i, db.calls[0].args[i], want)
		}
	}
}

// 200 and not 204, like every other delete in the app: base.templ's noSwap
// config lists 204, and a status in that list would stop the swap that puts the
// form back in the dialog.
func TestRevokingAShareAnswers200AndSwapsTheFormBack(t *testing.T) {
	app, _ := newPanelApp(1)

	rec := journalRequest(t, app.RevokeJournalShare, http.MethodDelete, nil, testEntryID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Create link") {
		t.Errorf("the reply is not the create form:\n%s", body)
	}
	if strings.Contains(body, "Revoke link") {
		t.Errorf("the reply still offers to revoke:\n%s", body)
	}
}

// Zero matched rows is a link that was already gone -- revoked in another tab,
// or an entry that is not this user's. Both are the same 404.
func TestRevokingAShareThatIsNotThereIs404(t *testing.T) {
	app, _ := newPanelApp(0)

	rec := journalRequest(t, app.RevokeJournalShare, http.MethodDelete, nil, testEntryID.String())

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTheShareLinkIsAbsoluteAndCarriesTheToken(t *testing.T) {
	cases := map[string]struct {
		forwarded string
		want      string
	}{
		"plain http":     {"", "http://tabletopper.test/share/abc"},
		"behind a proxy": {"https", "https://tabletopper.test/share/abc"},
		// The header is client-supplied when nothing is in front, so only the
		// two real schemes are taken from it.
		"a nonsense scheme": {"gopher", "http://tabletopper.test/share/abc"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/fragment/character/journal-share", nil)
			r.Host = "tabletopper.test"
			if tc.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", tc.forwarded)
			}

			if got := shareLink(r, "abc"); got != tc.want {
				t.Errorf("shareLink = %q, want %q", got, tc.want)
			}
		})
	}
}

// THE ONE THAT MATTERS. A body holds whatever was typed into it, and this is
// what decides which of those URLs a stranger's browser is asked to fetch.
func TestOnlyThisEntrysOwnImagesSurviveAShareRender(t *testing.T) {
	otherEntry := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS2")
	assetID := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS3")
	source := shareImageSource("tok", testCharacterID, testEntryID)

	mine := journalImagePath(testCharacterID, testEntryID, assetID)
	if got, ok := source(mine); !ok || got != "/share/tok/images/"+assetID.String() {
		t.Errorf("this entry's own image did not survive: %q %v", got, ok)
	}

	dropped := map[string]string{
		"another entry's image": journalImagePath(testCharacterID, otherEntry, assetID),
		"another character's":   journalImagePath(otherEntry, testEntryID, assetID),
		"the owner's avatar":    "/assets/images/" + assetID.String(),
		"somebody's tracker":    "https://tracker.example/pixel.gif",
		"a protocol-relative":   "//tracker.example/pixel.gif",
		"a data url":            "data:image/png;base64,AAAA",
		"the prefix alone":      journalImagePrefix(testCharacterID, testEntryID),
		"a traversal":           journalImagePrefix(testCharacterID, testEntryID) + "../../../etc",
		"a trailing segment":    mine + "/extra",
	}
	for name, dest := range dropped {
		if got, ok := source(dest); ok {
			t.Errorf("%s was served as %q", name, got)
		}
	}
}

// A token that is not shaped like one of ours is refused before a query runs.
func TestOnlyAWellShapedTokenIsWorthAQuery(t *testing.T) {
	token, err := share.NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if !share.ValidToken(token) {
		t.Errorf("a freshly minted token was refused: %q", token)
	}

	for _, bad := range []string{"", "short", token + "x", token[:21] + "+", token[:21] + "/", token[:21] + "="} {
		if share.ValidToken(bad) {
			t.Errorf("ValidToken accepted %q", bad)
		}
	}
}
