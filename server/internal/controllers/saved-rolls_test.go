package controllers

import (
	"bytes"
	"database/sql/driver"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/internal/uievents"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

var (
	testSavedRollID      = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVR0")
	testOtherSavedRollID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVR1")
)

func savedRollsAnswer(rows ...[3]string) roomAnswer {
	answer := roomAnswer{columns: []string{"id", "label", "expr"}}
	for i, row := range rows {
		values := []driver.Value{ulid.MustParse(row[0]).Bytes(), row[1], row[2]}
		if i == 0 {
			answer.values = values
			continue
		}
		answer.more = append(answer.more, values)
	}
	return answer
}
func savedRollsDB(saved roomAnswer) *roomDB {
	return &roomDB{
		rows:    1,
		answers: []roomAnswer{getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false)},
		named:   map[string]roomAnswer{"ListSavedRolls": saved},
	}
}
func saveRoll(t *testing.T, db *roomDB, sess session.UserSession, form url.Values) (int, string, string) {
	t.Helper()
	rec := tableRequest(t, newRoomApp(db).SaveRoll, http.MethodPost, "/rooms/"+testRoomID.String()+"/saved-rolls",
		map[string]string{"id": testRoomID.String()}, form, sess)
	return rec.Code, rec.Body.String(), rec.Header().Get("HX-Trigger")
}
func savedRollInserts(db *roomDB) []recordedCall {
	var inserts []recordedCall
	for _, call := range db.recorded() {
		if strings.Contains(call.query, "INSERT INTO saved_rolls") {
			inserts = append(inserts, call)
		}
	}
	return inserts
}
func boundTo(arg any, id ulid.ULID) bool {
	raw, ok := arg.([]byte)
	return ok && bytes.Equal(raw, id.Bytes())
}

func TestASavedRollIsKeptForTheAccountAndTheTrayIsToldToShowIt(t *testing.T) {
	db := savedRollsDB(savedRollsAnswer())
	form := url.Values{"expr": {"1D8  +  4"}, "label": {"  Longsword "}}
	code, body, trigger := saveRoll(t, db, memberSession(testRoomID), form)
	if code != http.StatusNoContent || body != "" {
		t.Fatalf("the save answered %d with %q, want 204 and nothing", code, body)
	}
	inserts := savedRollInserts(db)
	if len(inserts) != 1 {
		t.Fatalf("inserts = %d, want 1: %v", len(inserts), db.queries())
	}
	args := inserts[0].args
	if len(args) != 4 || !boundTo(args[1], testMemberID) || args[2] != "Longsword" || args[3] != "1d8 + 4" {
		t.Errorf("the roll was saved as %v, want the player's tidied Longsword 1d8 + 4", args)
	}
	raw, _ := args[0].([]byte)
	var id ulid.ULID
	if id.UnmarshalBinary(raw) != nil {
		t.Errorf("the saved roll's id %v is not a ULID", args[0])
	}
	for _, want := range []string{uievents.ModalClose, uievents.SavedRolls, "Longsword is saved."} {
		if !strings.Contains(trigger, want) {
			t.Errorf("the save did not send %q: %s", want, trigger)
		}
	}
}
func TestASaveTheModalCannotKeepIsExplainedInTheModal(t *testing.T) {
	var full [][3]string
	for i := range savedRollsMax {
		full = append(full, [3]string{ulid.Make().String(), "Roll " + strconv.Itoa(i), "1d20"})
	}
	for name, c := range map[string]struct {
		saved roomAnswer
		form  url.Values
		want  string
	}{
		"bad dice":      {savedRollsAnswer(), url.Values{"expr": {"2d6 * 3"}}, "multiply"},
		"no dice":       {savedRollsAnswer(), url.Values{"expr": {"  "}}, "Type something to roll"},
		"already saved": {savedRollsAnswer([3]string{testSavedRollID.String(), "Longsword", "1d8 + 4"}), url.Values{"expr": {"1d8+4"}, "label": {"longsword"}}, "already saved"},
		"a full list":   {savedRollsAnswer(full...), url.Values{"expr": {"1d6"}}, "keep 24 saved rolls"},
	} {
		db := savedRollsDB(c.saved)
		code, body, trigger := saveRoll(t, db, memberSession(testRoomID), c.form)
		if code != http.StatusUnprocessableEntity || !strings.Contains(body, `id="errors-`+pages.SaveRollPanel+`"`) {
			t.Errorf("%s answered %d with %q, want 422 and the modal's error block", name, code, body)
		}
		if !strings.Contains(html.UnescapeString(body), c.want) {
			t.Errorf("%s did not say %q: %s", name, c.want, body)
		}
		if strings.Contains(trigger, uievents.ModalClose) {
			t.Errorf("%s closed the modal", name)
		}
		if len(savedRollInserts(db)) != 0 {
			t.Errorf("%s was saved anyway", name)
		}
	}
}
func TestTheSameDiceUnderAnotherLabelAreSaved(t *testing.T) {
	db := savedRollsDB(savedRollsAnswer([3]string{testSavedRollID.String(), "Longsword", "1d8 + 4"}))
	code, _, _ := saveRoll(t, db, memberSession(testRoomID), url.Values{"expr": {"1d8 + 4"}, "label": {"Shortsword"}})
	if code != http.StatusNoContent || len(savedRollInserts(db)) != 1 {
		t.Errorf("the same dice under another label answered %d and were not saved", code)
	}
}
func saveRollModal(t *testing.T, sess session.UserSession, query url.Values) (int, string) {
	t.Helper()
	db := savedRollsDB(savedRollsAnswer())
	rec := tableRequest(t, newRoomApp(db).RoomSaveRollFragment, http.MethodGet,
		"/fragment/room/dice/save?"+query.Encode(), nil, nil, sess)
	return rec.Code, rec.Body.String()
}
func TestTheSaveModalCarriesTheDiceTypedInTheTray(t *testing.T) {
	code, body := saveRollModal(t, memberSession(testRoomID), url.Values{"room": {testRoomID.String()}, "expr": {"1D8  + 4"}})
	if code != http.StatusOK {
		t.Fatalf("the modal answered %d, want 200", code)
	}
	for _, want := range []string{
		`name="expr" value="1d8 + 4"`,
		`name="label"`,
		`hx-post="/rooms/` + testRoomID.String() + `/saved-rolls"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the modal is missing %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "disabled") {
		t.Error("a good roll cannot be saved from the modal")
	}
}
func TestTheSaveModalExplainsDiceItCannotSave(t *testing.T) {
	code, body := saveRollModal(t, memberSession(testRoomID), url.Values{"room": {testRoomID.String()}, "expr": {"2d6 * 3"}})
	if code != http.StatusOK || !strings.Contains(body, "multiply") {
		t.Errorf("the modal answered %d without the reason:\n%s", code, body)
	}
	if !strings.Contains(body, `class="btn btn-primary" disabled`) {
		t.Error("the modal offers to save dice it already knows it will refuse")
	}
}
func TestTheSaveModalIsNotFoundAwayFromTheTable(t *testing.T) {
	stranger := session.UserSession{UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVR9")}
	for name, c := range map[string]struct {
		sess  session.UserSession
		query url.Values
	}{
		"a stranger":       {stranger, url.Values{"room": {testRoomID.String()}, "expr": {"1d20"}}},
		"no room":          {memberSession(testRoomID), url.Values{"expr": {"1d20"}}},
		"an overlong roll": {memberSession(testRoomID), url.Values{"room": {testRoomID.String()}, "expr": {strings.Repeat("1", room.DiceExprLimit+1)}}},
	} {
		if code, body := saveRollModal(t, c.sess, c.query); code != http.StatusNotFound || body != "" {
			t.Errorf("%s answered %d with %q, want a bare 404", name, code, body)
		}
	}
}
func TestOnlySomebodyAtTheTableSavesFromItsTray(t *testing.T) {
	db := savedRollsDB(savedRollsAnswer())
	stranger := session.UserSession{UserID: ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVR9")}
	code, _, _ := saveRoll(t, db, stranger, url.Values{"expr": {"1d20"}})
	if code != http.StatusNotFound {
		t.Errorf("a stranger's save answered %d, want 404", code)
	}
	if got := db.queries(); len(got) != 1 {
		t.Errorf("a stranger's save ran %v, want only the room lookup", got)
	}
}
func TestRemovingASavedRollOnlyReachesTheOwnersRow(t *testing.T) {
	db := &roomDB{rows: 1}
	rec := tableRequest(t, newRoomApp(db).DeleteSavedRoll, http.MethodDelete, "/saved-rolls/"+testSavedRollID.String(),
		map[string]string{"id": testSavedRollID.String()}, nil, memberSession(testRoomID))
	if rec.Code != http.StatusOK {
		t.Fatalf("the removal returned %d, want 200 so the button is swapped away", rec.Code)
	}
	call := db.only(t)
	if !strings.Contains(call.query, "DELETE FROM saved_rolls") || len(call.args) != 2 ||
		!boundTo(call.args[0], testSavedRollID) || !boundTo(call.args[1], testMemberID) {
		t.Errorf("the removal ran %s %v, want the row bound to its owner", call.query, call.args)
	}
	bad := tableRequest(t, newRoomApp(&roomDB{}).DeleteSavedRoll, http.MethodDelete, "/saved-rolls/nope",
		map[string]string{"id": "nope"}, nil, memberSession(testRoomID))
	if bad.Code != http.StatusNotFound {
		t.Errorf("an unreadable id answered %d, want 404", bad.Code)
	}
}
func TestTheTrayShowsTheViewersSavedRollsInTheOrderTheyWereSaved(t *testing.T) {
	db := &roomDB{rows: 1, named: map[string]roomAnswer{"ListSavedRolls": savedRollsAnswer(
		[3]string{testSavedRollID.String(), "Longsword", "1d8 + 4"},
		[3]string{testOtherSavedRollID.String(), "", "1d20"},
	)}}
	for range diceReads {
		db.answers = append(db.answers, getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false))
	}
	app := liveRoomApp(t, db)
	markup := tray(t, app, memberSession(testRoomID))
	first := strings.Index(markup, `data-saved-roll="`+testSavedRollID.String()+`"`)
	second := strings.Index(markup, `data-saved-roll="`+testOtherSavedRollID.String()+`"`)
	if first < 0 || second < first {
		t.Errorf("the saved rolls are missing or out of order:\n%s", markup)
	}
	for _, call := range db.recorded() {
		if strings.Contains(call.query, "-- name: ListSavedRolls ") && (len(call.args) != 1 || !boundTo(call.args[0], testMemberID)) {
			t.Errorf("the tray listed saved rolls for %v, want the viewer's", call.args)
		}
	}
}
