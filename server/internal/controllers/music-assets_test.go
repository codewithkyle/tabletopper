package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
)

// startUpload posts one begin-upload request and SWALLOWS THE PANIC THAT
// FOLLOWS A SUCCESSFUL ONE.
//
// Every App here has a nil Storage, which is deliberate: presigning is the step
// straight after the insert, so a request that reaches it panics rather than
// signing anything. That is what these tests are built on -- a refusal returns
// cleanly and can be read off the recorder, and a request that got through is
// visible as the panic. Recovering here is what lets both be written the same
// way.
func startUpload(t *testing.T, app *App, body string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/assets/music", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	rec := httptest.NewRecorder()
	func() {
		defer func() { _ = recover() }()
		app.StartMusicUpload(rec, r)
	}()

	return rec
}

// EVERY REFUSAL HAPPENS BEFORE ANYTHING IS WRITTEN, and the nil Storage on this
// App is half of what proves it: presigning is reached only after the insert, so
// a request that got that far would have panicked instead of answering.
//
// The other half is the statement count. A name or a size this refuses costs the
// uploader nothing -- no row claiming a key, no signed URL, and the file never
// leaves their machine -- which matters here more than usual, because the thing
// being refused may be 175 MB.
func TestAMusicUploadIsRefusedBeforeAnyRowIsWritten(t *testing.T) {
	for name, c := range map[string]struct {
		body   string
		status int
	}{
		"an image":        {`{"name":"battle.png","size":100}`, http.StatusUnsupportedMediaType},
		"no extension":    {`{"name":"battle","size":100}`, http.StatusUnsupportedMediaType},
		"bare aac":        {`{"name":"battle.aac","size":100}`, http.StatusUnsupportedMediaType},
		"an empty file":   {`{"name":"battle.mp3","size":0}`, http.StatusBadRequest},
		"a negative size": {`{"name":"battle.mp3","size":-1}`, http.StatusBadRequest},
		"over the cap":    {`{"name":"battle.mp3","size":268435457}`, http.StatusRequestEntityTooLarge},
		"not even JSON":   {`nonsense`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			db := &recordingDB{}
			rec := startUpload(t, &App{Queries: queries.New(db)}, c.body)

			if rec.Code != c.status {
				t.Errorf("status = %d, want %d", rec.Code, c.status)
			}
			if len(db.calls) != 0 {
				t.Errorf("wrote %d statements before refusing: %v", len(db.calls), db.calls)
			}

			// THE BODY IS THE ALERT DIALOG'S OWN SHAPE. This route answers a
			// plain fetch(), which does not read HX-Trigger, so the refusal
			// travels in the body and public/js/music-upload.js dispatches it
			// as the same "alert" event the server-driven path raises.
			var problem jsonProblem
			if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
				t.Fatalf("the refusal is not JSON: %v", err)
			}
			if problem.Heading == "" || problem.Message == "" {
				t.Errorf("the refusal has nothing to show the user: %+v", problem)
			}
		})
	}
}

// The cap is checked at the boundary rather than near it: exactly the cap is
// allowed through, and one byte more is not.
func TestTheMusicCapIsInclusive(t *testing.T) {
	db := &recordingDB{}
	rec := startUpload(t, &App{Queries: queries.New(db)}, `{"name":"battle.mp3","size":268435456}`)

	// It got past every check and reached the insert, which is as far as it can
	// go here -- the presign that follows needs an R2 client this App has not
	// got.
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want the insert: %v", len(db.calls), db.calls)
	}
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Error("a track of exactly the cap was refused")
	}
}

// THE ROW GOES BEFORE THE SIGNED URL, which is the ledger rule in the one place
// it is easiest to get backwards. The URL names a key, and handing one out for a
// key no row claims would let the browser write an object nothing can play, no
// delete will find and no sweep will collect -- because every sweep works from
// these rows.
//
// It is born with uploaded_at NULL, so it owns a key and nothing else until the
// confirm has looked in the bucket. Nothing lists it and nothing plays it.
func TestAMusicRowClaimsItsKeyBeforeAURLIsSigned(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}

	startUpload(t, app, `{"name":"battle.mp3","size":1024}`)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	insert := db.calls[0]
	if !strings.Contains(insert.query, "INSERT INTO assets") {
		t.Fatalf("the first statement is not the insert: %q", insert.query)
	}
	if !strings.Contains(insert.query, "'music'") {
		t.Errorf("the row is not inserted as music: %q", insert.query)
	}
	if strings.Contains(insert.query, "uploaded_at") {
		t.Errorf("the row is born already confirmed, so it would be listed and played before its object exists: %q", insert.query)
	}

	// The key is the one MusicKey builds, and it is on the row -- so the
	// presigned URL and every later delete name the same object.
	var key string
	for _, arg := range insert.args {
		if s, ok := arg.(string); ok && strings.Contains(s, "/music/") {
			key = s
		}
	}
	if !strings.HasPrefix(key, "users/"+testOwnerID.String()+"/music/") {
		t.Errorf("the row claims %q, which is not this owner's music prefix", key)
	}
}

// A track that is still uploading is not a track. The listing filters on
// uploaded_at, so a row whose PUT is running -- or was abandoned when the tab
// closed -- never becomes a card, and a card is the only thing that renders a
// player.
func TestTheMusicLibraryListsOnlyFinishedUploads(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodGet, "/assets/music", nil)
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	app.MusicAssetsPage(httptest.NewRecorder(), r)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	list := db.calls[0]
	if !strings.Contains(list.query, "uploaded_at IS NOT NULL") {
		t.Errorf("the listing would draw a player for a track that is not there yet: %q", list.query)
	}
	if !strings.Contains(list.query, "owner_id = ?") {
		t.Errorf("the listing is not scoped to an owner: %q", list.query)
	}
}

// Renaming and deleting a track go through the same two handlers the pictures
// use, so the only thing that makes them a track's is the kind bound at the
// route. Without the type in the WHERE, a token's id sent to the music route
// would rename the token.
func TestMusicRenamesAreScopedToMusic(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodPatch, "/assets/music/x/name", strings.NewReader("name=Tavern"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", "01BX5ZZKBKACTAV9WEVGEMMVS2")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	app.RenameMusic(newRecorder(), r)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	var bound queries.AssetsType
	for _, arg := range db.calls[0].args {
		if v, ok := arg.(queries.AssetsType); ok {
			bound = v
		}
	}
	if bound != queries.AssetsTypeMusic {
		t.Errorf("the rename is scoped to %q, want music", bound)
	}
}
