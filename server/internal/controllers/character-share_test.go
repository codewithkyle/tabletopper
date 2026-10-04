package controllers

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func TestARejectedCharacterShareFormRunsNoStatements(t *testing.T) {
	for name, form := range map[string]map[string]string{
		"expiry with no days":  {"expiry": "on", "days": ""},
		"expiry past the cap":  {"expiry": "on", "days": strconv.Itoa(pages.ShareMaxDays + 1)},
		"expiry under the cap": {"expiry": "on", "days": strconv.Itoa(pages.ShareMinDays - 1)},
		"password that is one": {"protect": "on", "password": strings.Repeat("a", pages.SharePasswordMin-1)},
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

func shareRowAnswer() roomAnswer {
	return roomAnswer{
		columns: []string{
			"id", "owner_id", "character_id", "resource_type", "resource_id",
			"token", "password_hash", "expires_at", "created_at", "updated_at",
		},
		values: []driver.Value{
			testCharacterID[:], testOwnerID[:], testCharacterID[:], "character", testCharacterID[:],
			"AbCdEfGhIjKlMnOpQrStUv",
			"$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123",
			time.Now().Add(72 * time.Hour), time.Now(), time.Now(),
		},
	}
}
func createShareRequest(t *testing.T, form url.Values) (*httptest.ResponseRecorder, *roomDB) {
	t.Helper()
	db := &roomDB{rows: 1, answers: []roomAnswer{shareRowAnswer()}}
	app := &App{Queries: queries.New(db.db())}
	r := httptest.NewRequest(http.MethodPost, "/characters/x/share", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", testCharacterID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	app.CreateCharacterShare(rec, r)
	return rec, db
}
func TestAnExpiringPasswordedShareAnswersWithItsLink(t *testing.T) {
	rec, db := createShareRequest(t, shareForm(map[string]string{
		"expiry": "on", "days": "3",
		"protect": "on", "password": strings.Repeat("a", pages.SharePasswordMin),
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200:\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/share/AbCdEfGhIjKlMnOpQrStUv") {
		t.Errorf("the reply carries no link to copy:\n%s", body)
	}
	if !strings.Contains(body, "Revoke link") || strings.Contains(body, "Create link") {
		t.Errorf("the reply is still the form rather than the link:\n%s", body)
	}
	if got := rec.Header().Get("HX-Trigger"); !strings.Contains(got, "Share link created") {
		t.Errorf("HX-Trigger = %q, want the toast", got)
	}
	if len(db.queries()) != 2 {
		t.Errorf("statements run = %v, want the insert and the read back", db.queries())
	}
	expiry, ok := writtenExpiry(db.recorded()[0])
	if !ok {
		t.Fatalf("the insert wrote no expiry: %#v", db.recorded()[0].args)
	}
	if days := time.Until(expiry).Hours() / 24; days < 2.9 || days > 3.1 {
		t.Errorf("the link expires in %.2f days, want the 3 that were asked for", days)
	}
}
func TestTheLongestAndShortestExpiryTheFieldOffersAreBothAccepted(t *testing.T) {
	for _, days := range []int{pages.ShareMinDays, pages.ShareMaxDays} {
		t.Run(strconv.Itoa(days), func(t *testing.T) {
			rec, _ := createShareRequest(t, shareForm(map[string]string{
				"expiry": "on", "days": strconv.Itoa(days),
			}))
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d for %d days, want 200: the field offers an expiry the save refuses",
					rec.Code, days)
			}
		})
	}
}
func writtenExpiry(call recordedCall) (time.Time, bool) {
	for _, arg := range call.args {
		if at, ok := arg.(time.Time); ok {
			return at, true
		}
	}
	return time.Time{}, false
}
