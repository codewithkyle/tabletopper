package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/internal/sweep"

	"github.com/oklog/ulid/v2"
)

func anAsset(id ulid.ULID, kind queries.AssetsType, key, name string) roomAnswer {
	return roomAnswer{
		columns: []string{
			"id", "owner_id", "journal_id", "file_path", "preview_path", "type",
			"file_name", "size_bytes", "name", "detached_at", "width", "height",
			"tile_size", "max_zoom", "tile_gen", "tile_state", "tile_attempts",
			"tile_lease", "tile_leased_at", "tiled_at", "uploaded_at",
			"created_at", "updated_at",
		},
		values: []driver.Value{
			id.Bytes(), testOwnerID.Bytes(), nil, key, nil, string(kind),
			name, int64(1 << 10), name, nil, nil, nil,
			nil, nil, nil, nil, int64(0),
			nil, nil, nil, nil,
			time.Unix(0, 0), time.Unix(0, 0),
		},
	}
}
func anOwnedImage(name string, image *ulid.ULID, key string) roomAnswer {
	answer := roomAnswer{
		columns: []string{"name", "asset_id", "file_path"},
		values:  []driver.Value{name, nil, nil},
	}
	if image != nil {
		answer.values[1] = image.Bytes()
		answer.values[2] = key
	}
	return answer
}

type queuedPurge struct {
	key    string
	prefix bool
}

func purgesQueued(t *testing.T, db *roomDB) []queuedPurge {
	t.Helper()
	var queued []queuedPurge
	for _, call := range db.recorded() {
		if !strings.Contains(call.query, "INSERT INTO object_purge") {
			continue
		}
		if len(call.args) != 3 {
			t.Fatalf("the purge binds %d values, want id, key and prefix: %v", len(call.args), call.args)
		}
		key, _ := call.args[1].(string)
		prefix, _ := call.args[2].(bool)
		queued = append(queued, queuedPurge{key: key, prefix: prefix})
	}
	return queued
}
func assertOneTransaction(t *testing.T, db *roomDB, statements ...string) {
	t.Helper()
	steps := db.recordedSteps()
	begin, commit := -1, -1
	for i, step := range steps {
		switch step {
		case "BEGIN":
			if begin >= 0 {
				t.Fatalf("more than one transaction ran: %v", steps)
			}
			begin = i
		case "COMMIT":
			commit = i
		case "ROLLBACK":
			t.Fatalf("the transaction rolled back: %v", steps)
		}
	}
	if begin < 0 || commit < begin {
		t.Fatalf("no committed transaction: %v", steps)
	}
	for _, statement := range statements {
		found := false
		for i, step := range steps {
			if !strings.Contains(step, statement) {
				continue
			}
			found = true
			if i < begin || i > commit {
				t.Errorf("%q ran outside the transaction", statement)
			}
		}
		if !found {
			t.Errorf("%q never ran: %v", statement, db.queries())
		}
	}
}
func deleteRequest(t *testing.T, app *App, handler func(*App) http.HandlerFunc, id ulid.ULID) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodDelete, "/assets/x/"+id.String(), nil)
	r.SetPathValue("id", id.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()
	handler(app)(rec, r)
	return rec
}
func TestEveryDeleteQueuesItsObjectsInsideTheTransactionThatDeletesTheRows(t *testing.T) {
	portrait := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVP1")
	monsterArt := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVP2")
	portraitKey := storage.CharacterPortraitKey(testOwnerID, portrait)
	journalKey := storage.JournalImageKey(testOwnerID, testAssetID)
	monsterKey := storage.MonsterImageKey(testOwnerID, monsterArt)
	tokenKey := storage.TokenKey(testOwnerID, testAssetID)
	for name, c := range map[string]struct {
		handler func(*App) http.HandlerFunc
		id      ulid.ULID
		answers []roomAnswer
		want    []queuedPurge
		deletes []string
	}{
		"a map": {
			handler: func(a *App) http.HandlerFunc { return a.DeleteMap },
			id:      testAssetID,
			answers: []roomAnswer{anAsset(testAssetID, queries.AssetsTypeMap, storage.MapOriginalKey(testOwnerID, testAssetID), "Phandalin")},
			want:    []queuedPurge{{key: storage.MapPrefix(testOwnerID, testAssetID), prefix: true}},
			deletes: []string{"DELETE FROM assets"},
		},
		"a token": {
			handler: func(a *App) http.HandlerFunc { return a.DeleteToken },
			id:      testAssetID,
			answers: []roomAnswer{anAsset(testAssetID, queries.AssetsTypeToken, tokenKey, "Goblin")},
			want:    []queuedPurge{{key: tokenKey}},
			deletes: []string{"DELETE FROM assets"},
		},
		"a monster": {
			handler: func(a *App) http.HandlerFunc { return a.DeleteMonster },
			id:      testMonsterID,
			answers: []roomAnswer{anOwnedImage("Owlbear", &monsterArt, monsterKey)},
			want:    []queuedPurge{{key: monsterKey}},
			deletes: []string{"DELETE FROM monsters", "DELETE FROM assets"},
		},
		"a character": {
			handler: func(a *App) http.HandlerFunc { return a.DeleteCharacter },
			id:      testCharacterID,
			answers: []roomAnswer{
				anOwnedImage("Ilyana", &portrait, portraitKey),
				{columns: []string{"file_path"}, values: []driver.Value{journalKey}},
			},
			want:    []queuedPurge{{key: journalKey}, {key: portraitKey}},
			deletes: []string{"DELETE FROM characters", "DELETE FROM assets"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			db := &roomDB{rows: 1, answers: c.answers}
			rec := deleteRequest(t, newRoomApp(db), c.handler, c.id)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; an R2 call on the request path would have panicked on the nil Storage", rec.Code, http.StatusOK)
			}
			got := purgesQueued(t, db)
			if len(got) != len(c.want) {
				t.Fatalf("queued %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("purge %d = %+v, want %+v", i, got[i], c.want[i])
				}
			}
			assertOneTransaction(t, db, append([]string{"INSERT INTO object_purge"}, c.deletes...)...)
		})
	}
}
func TestADeleteWithNoImageQueuesNothing(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{anOwnedImage("Owlbear", nil, "")}}
	rec := deleteRequest(t, newRoomApp(db), func(a *App) http.HandlerFunc { return a.DeleteMonster }, testMonsterID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := purgesQueued(t, db); len(got) != 0 {
		t.Errorf("queued %v for a monster with no image", got)
	}
}
func TestADiscardedUploadDropsTheRowAndQueuesTheObjectTogether(t *testing.T) {
	db := &roomDB{rows: 1}
	app := newRoomApp(db)
	key := storage.AvatarKey(testOwnerID, testAssetID)
	app.discardAsset(context.Background(), testOwnerID, testAssetID, sweep.Key(key))
	if got := purgesQueued(t, db); len(got) != 1 || got[0] != (queuedPurge{key: key}) {
		t.Errorf("queued %v, want the avatar key", got)
	}
	assertOneTransaction(t, db, "DELETE FROM assets", "INSERT INTO object_purge")
}
func TestARowOnlyDiscardQueuesNothing(t *testing.T) {
	db := &roomDB{rows: 1}
	newRoomApp(db).discardAssetRow(context.Background(), testOwnerID, testAssetID)
	if got := purgesQueued(t, db); len(got) != 0 {
		t.Errorf("queued %v for a row with no object", got)
	}
	assertOneTransaction(t, db, "DELETE FROM assets")
}
func TestRenamingAnAssetThatIsGoneSaysSo(t *testing.T) {
	cases := map[string]func(*App, http.ResponseWriter, *http.Request){
		"a map":   func(a *App, w http.ResponseWriter, r *http.Request) { a.ReplaceMapName(w, r) },
		"a token": func(a *App, w http.ResponseWriter, r *http.Request) { a.renameLibrary(w, r, tokenKind.assetKind) },
	}
	for name, rename := range cases {
		for _, matched := range []int64{0, 1} {
			db := &recordingDB{rows: matched}
			app := &App{Queries: queries.New(db)}
			r := httptest.NewRequest(http.MethodPatch, "/assets/x/"+testAssetID.String()+"/name", strings.NewReader("name=Rowboat&map-name=Rowboat"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.SetPathValue("id", testAssetID.String())
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			rec := httptest.NewRecorder()
			rename(app, rec, r)
			trigger := rec.Header().Get("HX-Trigger")
			if matched == 0 {
				if rec.Code != http.StatusNotFound || !strings.Contains(trigger, "no longer exists") {
					t.Errorf("%s: renaming a deleted asset answered %d %q, want the not-found alert", name, rec.Code, trigger)
				}
				continue
			}
			if rec.Code != http.StatusOK || !strings.Contains(trigger, "Rowboat updated.") {
				t.Errorf("%s: a rename answered %d %q, want the toast", name, rec.Code, trigger)
			}
		}
	}
}
func TestOnlyRecentlyDetachedImagesCountAgainstTheEntry(t *testing.T) {
	app, db := newPanelApp(1)
	r := httptest.NewRequest(http.MethodPost, "/characters/x/journal/y/images", nil)
	r.SetPathValue("id", testCharacterID.String())
	r.SetPathValue("entryId", testEntryID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	before := time.Now()
	app.UploadJournalImage(httptest.NewRecorder(), r)
	if len(db.reads) == 0 {
		t.Fatal("the upload never counted the entry's images")
	}
	count := db.reads[0]
	if !strings.Contains(count.query, "a.detached_at IS NULL OR a.detached_at > ?") {
		t.Errorf("the count does not drop images detached long ago:\n%s", count.query)
	}
	recent, ok := count.args[0].(sql.NullTime)
	if !ok || !recent.Valid {
		t.Fatalf("the first bound value is %#v, want the cutoff", count.args[0])
	}
	if age := before.Sub(recent.Time); age < 59*time.Minute || age > 61*time.Minute {
		t.Errorf("the cutoff is %v before the request, want about an hour", age)
	}
}
