package controllers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

// The sheet's share, tested the way the entry's is next door and bounded the
// same way: recordingDB answers a :one by panicking, so the dialog fragment and
// the shared page itself cannot be driven from this harness. What is covered is
// the validation the create runs before it touches the database, the revoke, and
// the sentence the dialog puts in front of the person deciding to hand the link
// out.

// A rejected create must not have written anything, and the reason is sharper
// here than for an entry: the token is minted and the password hashed after this
// point, so a form that gets past validation is one that is going to be
// inserted.
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

// REVOKING THE SHEET'S LINK IS NOT REVOKING THE CHARACTER'S LINKS. The statement
// pins resource_type, so every journal entry this character shared is still
// readable afterwards -- which is the difference between DeleteCharacterShare
// and DeleteSharesForCharacter, two statements a letter apart that the character
// delete and this handler must not swap.
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

// 200 and not 204, like every other delete in the app: base.templ's noSwap
// config lists 204, and a status in that list would stop the swap that puts the
// form back in the dialog.
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

// Zero matched rows is a link that was already gone -- revoked on another tab of
// the same editor, or a character that is not this user's. Both are the same 404.
func TestRevokingACharacterShareThatIsNotThereIs404(t *testing.T) {
	app, _ := newPanelApp(0)

	rec := journalRequest(t, app.RevokeCharacterShare, http.MethodDelete, nil, "")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// THE DIALOG HAS TO SAY WHAT THE LINK DOES NOT INCLUDE. What a shared sheet
// shows is decided in share-character.go, and the one thing an owner is likely
// to assume travels with a character is its journal -- which does not, because
// an entry has its own link, its own expiry and its own password. Saying so is
// the difference between a scope decision and a surprise.
func TestTheCharacterShareDialogSaysTheJournalIsNotIncluded(t *testing.T) {
	data := characterShareDialogData(testCharacterID)

	if data.Action != "/characters/"+testCharacterID.String()+"/share" {
		t.Errorf("the dialog acts on %q", data.Action)
	}
	if !strings.Contains(strings.ToLower(data.Blurb), "journal") {
		t.Errorf("the blurb does not mention the journal:\n%s", data.Blurb)
	}
}
