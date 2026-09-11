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














func shareForm(values map[string]string) url.Values {
	form := url.Values{}
	for name, value := range values {
		form.Set(name, value)
	}

	return form
}




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
		if got, ok := boundID(db.calls[0].args[i]); !ok || got != want {
			t.Errorf("revoke arg %d = %v, want %v", i, db.calls[0].args[i], want)
		}
	}
}




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
		"plain http":     {"", "http:
		"behind a proxy": {"https", "https:
		
		
		"a nonsense scheme": {"gopher", "http:
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
		"somebody's tracker":    "https:
		"a protocol-relative":   "
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
