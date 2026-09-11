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
func TestTheMusicCapIsInclusive(t *testing.T) {
	db := &recordingDB{}
	rec := startUpload(t, &App{Queries: queries.New(db)}, `{"name":"battle.mp3","size":268435456}`)
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want the insert: %v", len(db.calls), db.calls)
	}
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Error("a track of exactly the cap was refused")
	}
}
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
