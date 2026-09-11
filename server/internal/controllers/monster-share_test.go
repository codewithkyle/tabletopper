package controllers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"tabletopper/internal/session"
)

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
func TestTheMonsterShareDialogSaysACopyCannotBeTakenBack(t *testing.T) {
	blurb := monsterShareDialogData(testMonsterID).Blurb
	for _, want := range []string{"copy it into their own manual", "does not take it back"} {
		if !strings.Contains(blurb, want) {
			t.Errorf("the blurb does not say %q:\n%s", want, blurb)
		}
	}
}
