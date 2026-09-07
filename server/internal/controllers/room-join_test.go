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

// joinPost drives JoinRoomForm over a stub, with a session that carries the
// hash the join writes against.
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

// newJoinApp is an App whose one query answers with the room the code names.
// locked and closed are the two states a lookup can come back in; a room that
// is closed has no code and so cannot be found at all, which is why there is no
// closed case here.
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

// openRoomAnswer is the result set GetOpenRoomByCode reads.
func openRoomAnswer(id ulid.ULID, name string, locked bool) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "name", "is_locked"},
		values:  []driver.Value{id.Bytes(), name, locked},
	}
}

// A code that is not shaped like one cannot name a room, so it is refused
// before anything is queried -- the same refusal share.ValidToken makes in
// front of every share route.
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

			rec := joinPost(t, app, url.Values{"code": {code}})

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

// The stored code is upper case, so a player typing what their keyboard was in
// has to reach the statement in the shape the column holds.
func TestATypedCodeIsNormalisedBeforeTheLookup(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{openRoomAnswer(testRoomID, "Curse of Strahd", false)}}
	app := newJoinApp(db)

	rec := joinPost(t, app, url.Values{"code": {"  ab2c \n"}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) == 0 {
		t.Fatal("nothing was queried")
	}
	if got := boundCode(t, db.calls[0].args[0]); got != "AB2C" {
		t.Errorf("the lookup asked for %q, want %q", got, "AB2C")
	}
}

// THE COUNTER IS ASKED BEFORE THE ROOM IS LOOKED UP, which is what makes it a
// bound on guessing rather than a bound on being told the answer. A refused try
// still counts, so somebody hammering a locked window holds it locked.
func TestTheEleventhJoinInAMinuteIsRefusedBeforeTheLookup(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{openRoomAnswer(testRoomID, "Curse of Strahd", false)}}
	app := newJoinApp(db)

	for i := range 10 {
		if rec := joinPost(t, app, url.Values{"code": {"AB2C"}}); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("try %d was rate limited inside the limit", i+1)
		}
	}

	before := len(db.calls)
	rec := joinPost(t, app, url.Values{"code": {"AB2C"}})

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if len(db.calls) != before {
		t.Errorf("the refused try still ran %d statements", len(db.calls)-before)
	}
	if want := "Too many attempts. Wait a minute and try again."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
	// 429 rather than 422, and the form carries an hx-status route for it --
	// without one the noSwap list would swallow the message.
	if !strings.Contains(rec.Body.String(), `id="errors-join-room"`) {
		t.Errorf("body is not the form's error block: %s", rec.Body.String())
	}
}

// A LOCKED ROOM IS TOLD IT IS LOCKED, which is the one place this design leaks
// that a code is in use. A room code is not a bearer credential and the friend
// who typed the right code is who the message is for.
func TestALockedRoomSaysSo(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{openRoomAnswer(testRoomID, "Curse of Strahd", true)}}
	app := newJoinApp(db)

	rec := joinPost(t, app, url.Values{"code": {"AB2C"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if want := "That room is locked. Ask the GM to unlock it."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
	// The lookup ran and nothing else did: a locked room is not joined.
	if len(db.calls) != 1 {
		t.Errorf("ran %d statements, want 1: %v", len(db.calls), db.queries())
	}
}

// A code that names no open room says exactly that. It is a different sentence
// from the locked one on purpose -- the alternative is a player who typed the
// right code and cannot tell whether they typed it wrong.
func TestACodeThatNamesNoOpenRoomSaysSo(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{{columns: []string{"id", "name", "is_locked"}}}}
	app := newJoinApp(db)

	rec := joinPost(t, app, url.Values{"code": {"AB2C"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if want := "No open room has that code."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q: %s", want, rec.Body.String())
	}
}

// The join writes the session row, keyed on the hash, and answers with a
// redirect the toast rides along on.
func TestJoiningSeatsTheSessionAndRedirects(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{openRoomAnswer(testRoomID, "Curse of Strahd", false)}}
	app := newJoinApp(db)

	rec := joinPost(t, app, url.Values{"code": {"AB2C"}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want the lookup and the seat: %v", len(db.calls), db.queries())
	}

	seat := db.calls[1]
	if !strings.Contains(seat.query, "UPDATE sessions") || !strings.Contains(seat.query, "room_id = ?") {
		t.Errorf("the second statement is not the seat: %q", seat.query)
	}
	if len(seat.args) != 3 {
		t.Fatalf("the seat took %d values, want 3", len(seat.args))
	}
	if id, ok := boundRoomID(seat.args[0]); !ok || id != testRoomID {
		t.Errorf("the seat wrote room %v, want %v", seat.args[0], testRoomID)
	}
	if seat.args[1] != nil {
		t.Errorf("character = %v, want NULL for a join with no character", seat.args[1])
	}
	// Keyed on the hash, like every other statement against this row.
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

// THE CHARACTER IS VERIFIED BEFORE THE ROOM IS LOOKED UP, so a caller cannot
// use a bad character to probe codes without paying the rate limit -- and it is
// checked against the roster rather than trusted, because it is the id a pawn
// is spawned from later.
func TestACharacterThatIsNotYoursIsRefusedBeforeTheLookup(t *testing.T) {
	for name, character := range map[string]string{
		"not a ULID":      "nonsense",
		"somebody else's": ulid.Make().String(),
	} {
		t.Run(name, func(t *testing.T) {
			// No answers at all, so the character lookup -- if it runs --
			// finds nothing, which is what "not yours" means here.
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
