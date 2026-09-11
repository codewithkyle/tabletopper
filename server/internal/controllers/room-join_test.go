package controllers
import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"github.com/oklog/ulid/v2"
)
var testJoinCharacterID = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS0")
func joinForm(code string) url.Values {
	return url.Values{"code": {code}, "character": {testJoinCharacterID.String()}}
}
func characterAnswer(name string) roomAnswer {
	return roomAnswer{columns: []string{"name"}, values: []driver.Value{name}}
}
func joinPost(t *testing.T, app *App, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/rooms/join", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{
		UserID: testOwnerID,
		Hash:   []byte("session-hash"),
	}))
	rec := httptest.NewRecorder()
	app.JoinRoomForm(rec, r)
	return rec
}
func newJoinApp(db *roomDB) *App {
	pool := db.db()
	q := queries.New(pool)
	return &App{
		DB:               pool,
		Queries:          q,
		Sessions:         session.NewStore(q, false),
		RoomJoinAttempts: share.NewAttempts(10, time.Minute),
	}
}
func openRoomAnswer(id ulid.ULID, name string, locked bool) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "name", "is_locked"},
		values:  []driver.Value{id.Bytes(), name, locked},
	}
}
func TestAMalformedCodeIsRefusedWithoutAQuery(t *testing.T) {
	for name, code := range map[string]string{
		"three characters":                  "AB2",
		"five characters":                   "AB2CD",
		"empty":                             "",
		"a letter left out of the alphabet": "AB2O",
		"punctuation":                       "AB-2",
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1}
			app := newJoinApp(db)
			rec := joinPost(t, app, joinForm(code))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0: %v", len(db.calls), db.queries())
			}
			if want := "A room code is four letters or numbers."; !strings.Contains(rec.Body.String(), want) {
				t.Errorf("body missing %q: %s", want, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `id="errors-join-room"`) {
				t.Errorf("body is not the form's error block: %s", rec.Body.String())
			}
		})
	}
}
func TestATypedCodeIsNormalisedBeforeTheLookup(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		characterAnswer("Ilyana"),
		openRoomAnswer(testRoomID, "Curse of Strahd", false),
	}}
	app := newJoinApp(db)
	rec := joinPost(t, app, joinForm("  ab2c \n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) < 2 {
		t.Fatalf("ran %d statements, want the character and the lookup: %v", len(db.calls), db.queries())
	}
	if got := boundCode(t, db.calls[1].args[0]); got != "AB2C" {
		t.Errorf("the lookup asked for %q, want %q", got, "AB2C")
	}
}
func TestTheEleventhJoinInAMinuteIsRefusedBeforeTheLookup(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newJoinApp(db)
	for i := range 10 {
		if rec := joinPost(t, app, joinForm("AB2C")); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("try %d was rate limited inside the limit", i+1)
		}
	}
	before := len(db.calls)
	rec := joinPost(t, app, joinForm("AB2C"))
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if len(db.calls) != before {
		t.Errorf("the refused try still ran %d statements", len(db.calls)-before)
	}
	if want := "Too many attempts. Wait a minute and try again."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `id="errors-join-room"`) {
		t.Errorf("body is not the form's error block: %s", rec.Body.String())
	}
}
func TestALockedRoomSaysSo(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		characterAnswer("Ilyana"),
		openRoomAnswer(testRoomID, "Curse of Strahd", true),
	}}
	app := newJoinApp(db)
	rec := joinPost(t, app, joinForm("AB2C"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if want := "That room is locked. Ask the GM to unlock it."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
	if len(db.calls) != 2 {
		t.Errorf("ran %d statements, want 2: %v", len(db.calls), db.queries())
	}
}
func TestACodeThatNamesNoOpenRoomSaysSo(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		characterAnswer("Ilyana"),
		{columns: []string{"id", "name", "is_locked"}},
	}}
	app := newJoinApp(db)
	rec := joinPost(t, app, joinForm("AB2C"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if want := "No open room has that code."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
}
func TestJoiningSeatsTheSessionAndRedirects(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{
		characterAnswer("Ilyana"),
		openRoomAnswer(testRoomID, "Curse of Strahd", false),
	}}
	app := newJoinApp(db)
	rec := joinPost(t, app, joinForm("AB2C"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) != 3 {
		t.Fatalf("ran %d statements, want the character, the lookup and the seat: %v", len(db.calls), db.queries())
	}
	seat := db.calls[2]
	if !strings.Contains(seat.query, "UPDATE sessions") || !strings.Contains(seat.query, "room_id = ?") {
		t.Errorf("the third statement is not the seat: %q", seat.query)
	}
	if len(seat.args) != 3 {
		t.Fatalf("the seat took %d values, want 3", len(seat.args))
	}
	if id, ok := boundRoomID(seat.args[0]); !ok || id != testRoomID {
		t.Errorf("the seat wrote room %v, want %v", seat.args[0], testRoomID)
	}
	if id, ok := boundRoomID(seat.args[1]); !ok || id != testJoinCharacterID {
		t.Errorf("the seat wrote character %v, want %v", seat.args[1], testJoinCharacterID)
	}
	if hash, ok := seat.args[2].([]byte); !ok || string(hash) != "session-hash" {
		t.Errorf("the seat is keyed on %v, want the session hash", seat.args[2])
	}
	if want := "/rooms/" + testRoomID.String(); rec.Header().Get("HX-Redirect") != want {
		t.Errorf("HX-Redirect = %q, want %q", rec.Header().Get("HX-Redirect"), want)
	}
	if got := toastFrom(t, rec); got != "You joined Curse of Strahd." {
		t.Errorf("toast = %q", got)
	}
}
func TestACharacterThatIsNotYoursIsRefusedBeforeTheLookup(t *testing.T) {
	for name, character := range map[string]string{
		"not a ULID":      "nonsense",
		"somebody else's": ulid.Make().String(),
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1}
			app := newJoinApp(db)
			rec := joinPost(t, app, url.Values{"code": {"AB2C"}, "character": {character}})
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
			}
			if want := "That character is not yours."; !strings.Contains(rec.Body.String(), want) {
				t.Errorf("body missing %q: %s", want, rec.Body.String())
			}
			for _, sent := range db.queries() {
				if strings.Contains(sent, "rooms") {
					t.Errorf("the room was looked up anyway: %v", db.queries())
				}
			}
		})
	}
}
func TestAJoinWithNoCharacterIsRefusedBeforeTheLookup(t *testing.T) {
	for name, form := range map[string]url.Values{
		"an empty value": {"code": {"AB2C"}, "character": {""}},
		"no field":       {"code": {"AB2C"}},
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: []roomAnswer{
				characterAnswer("Ilyana"),
				openRoomAnswer(testRoomID, "Curse of Strahd", false),
			}}
			app := newJoinApp(db)
			rec := joinPost(t, app, form)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
			}
			if want := "Choose the character you are playing."; !strings.Contains(rec.Body.String(), want) {
				t.Errorf("body missing %q: %s", want, rec.Body.String())
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0: %v", len(db.calls), db.queries())
			}
		})
	}
}
