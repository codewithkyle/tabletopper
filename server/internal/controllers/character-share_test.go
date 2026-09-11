package controllers
import (
	"net/http"
	"strings"
	"testing"
	"github.com/oklog/ulid/v2"
)
func TestARejectedCharacterShareFormRunsNoStatements(t *testing.T) {
	for name, form := range map[string]map[string]string{
		"expiry with no days":  {"expiry": "on", "days": ""},
		"expiry past a year":   {"expiry": "on", "days": "366"},
		"password that is one": {"protect": "on", "password": "abc"},
	} {
		t.Run(name, func(t *testing.T) {
			app, db := newPanelApp(1)
			rec := journalRequest(t, app.CreateCharacterShare, http.MethodPost, shareForm(form), "")
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("a rejected form ran %d statements:\n%s", len(db.calls), db.calls[0].query)
			}
		})
	}
}
func TestRevokingTheSheetsLinkTouchesOnlyTheSheetsRow(t *testing.T) {
	app, db := newPanelApp(1)
	rec := journalRequest(t, app.RevokeCharacterShare, http.MethodDelete, nil, "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(db.calls) != 1 {
		t.Fatalf("statements run = %d, want 1", len(db.calls))
	}
	query := db.calls[0].query
	if !strings.Contains(query, "DELETE FROM shares") {
		t.Fatalf("did not delete from shares:\n%s", query)
	}
	if !strings.Contains(query, "resource_type = 'character'") {
		t.Errorf("the revoke is not pinned to the character share, so it can reach journal links:\n%s", query)
	}
	for i, want := range []ulid.ULID{testCharacterID, testOwnerID} {
		if got, ok := db.calls[0].args[i].(ulid.ULID); !ok || got != want {
			t.Errorf("revoke arg %d = %v, want %v", i, db.calls[0].args[i], want)
		}
	}
}
func TestRevokingACharacterShareAnswers200AndSwapsTheFormBack(t *testing.T) {
	app, _ := newPanelApp(1)
	rec := journalRequest(t, app.RevokeCharacterShare, http.MethodDelete, nil, "")
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
func TestRevokingACharacterShareThatIsNotThereIs404(t *testing.T) {
	app, _ := newPanelApp(0)
	rec := journalRequest(t, app.RevokeCharacterShare, http.MethodDelete, nil, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
func TestTheCharacterShareDialogSaysTheJournalIsNotIncluded(t *testing.T) {
	data := characterShareDialogData(testCharacterID)
	if data.Action != "/characters/"+testCharacterID.String()+"/share" {
		t.Errorf("the dialog acts on %q", data.Action)
	}
	if !strings.Contains(strings.ToLower(data.Blurb), "journal") {
		t.Errorf("the blurb does not mention the journal:\n%s", data.Blurb)
	}
}
