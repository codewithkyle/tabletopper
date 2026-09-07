package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/session"
)

// A monster's share, tested the way the sheet's is next door and bounded the
// same way: recordingDB answers a :one by failing, so the dialog fragment and
// the shared page itself cannot be driven from this harness. What is covered is
// the validation the create runs before it touches the database, the revoke, and
// the sentence the dialog puts in front of the person deciding to hand the link
// out -- which for a monster is the one that says what the link gives away
// permanently.

func monsterShareRequest(t *testing.T, handler http.HandlerFunc, method string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(method, "/monsters/share", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testMonsterID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	handler(rec, r)

	return rec
}

// A rejected create must not have written anything, and the reason is the one
// the sheet's version gives: the token is minted and the password hashed after
// this point, so a form that gets past validation is one that is going to be
// inserted.
func TestARejectedMonsterShareFormRunsNoStatements(t *testing.T) {
	for name, form := range map[string]map[string]string{
		"expiry with no days":  {"expiry": "on", "days": ""},
		"expiry past a year":   {"expiry": "on", "days": "366"},
		"password that is one": {"protect": "on", "password": "abc"},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)

			rec := monsterShareRequest(t, app.CreateMonsterShare, http.MethodPost, shareForm(form))

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("a rejected form ran %d statements:\n%s", len(db.calls), db.calls[0].query)
			}
		})
	}
}

// REVOKING A MONSTER'S LINK CANNOT REACH A CHARACTER'S ROW. resource_id is one
// column holding three kinds of id, so the type is what keeps them apart -- and
// a ULID naming a monster in this account could name a character in it too if
// the statement stopped pinning it.
func TestRevokingAMonsterShareTouchesOnlyAMonstersRow(t *testing.T) {
	app, db := newPanelApp(1)

	rec := monsterShareRequest(t, app.RevokeMonsterShare, http.MethodDelete, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	call := db.only(t)

	if !strings.Contains(call.query, "DELETE FROM shares") {
		t.Fatalf("did not delete from shares:\n%s", call.query)
	}
	if !strings.Contains(call.query, "resource_type = 'monster'") {
		t.Errorf("the revoke is not pinned to a monster's share:\n%s", call.query)
	}
	for i, want := range []string{testMonsterID.String(), testOwnerID.String()} {
		got, ok := boundID(call.args[i])
		if !ok || got.String() != want {
			t.Errorf("revoke arg %d = %v, want %v", i, call.args[i], want)
		}
	}
}

// 200 and not 204, like every other delete in the app: base.templ's noSwap
// config lists 204, and a status in that list would stop the swap that puts the
// form back in the dialog.
func TestRevokingAMonsterShareAnswers200AndSwapsTheFormBack(t *testing.T) {
	app, _ := newPanelApp(1)

	rec := monsterShareRequest(t, app.RevokeMonsterShare, http.MethodDelete, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Create link") {
		t.Errorf("the dialog did not come back as the form:\n%s", body)
	}
	if !strings.Contains(body, "/monsters/"+testMonsterID.String()+"/share") {
		t.Errorf("the form posts somewhere other than this monster's share:\n%s", body)
	}
}

// Zero matched rows is a link that was already gone -- revoked on another tab of
// the same editor, or a monster that is not this user's. Both are the same 404,
// and the alert says which thing was missing rather than naming the monster,
// because the monster is still there.
func TestRevokingAMonsterShareThatIsNotThereIs404(t *testing.T) {
	app, _ := newPanelApp(0)

	rec := monsterShareRequest(t, app.RevokeMonsterShare, http.MethodDelete, nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "share link") {
		t.Errorf("the alert does not name the share link: %s", trigger)
	}
}

// THE BLURB IS THE ONLY PLACE AN OWNER IS TOLD WHAT THE BUTTON GIVES AWAY. A
// shared sheet can only be read; a shared monster can be taken, and the copy is
// the reader's from that moment -- so revoking the link afterwards takes nothing
// back. Someone about to paste this URL into a table's chat is entitled to know
// that before they do rather than after.
func TestTheMonsterShareDialogSaysACopyCannotBeTakenBack(t *testing.T) {
	blurb := monsterShareDialogData(testMonsterID).Blurb

	for _, want := range []string{"copy it into their own manual", "does not take it back"} {
		if !strings.Contains(blurb, want) {
			t.Errorf("the blurb does not say %q:\n%s", want, blurb)
		}
	}
}
