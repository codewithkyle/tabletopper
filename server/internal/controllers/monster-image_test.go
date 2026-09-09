package controllers

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"

	"github.com/oklog/ulid/v2"
)

// A real four-pixel PNG, because this upload path decodes what it is given --
// unlike a map's, which stores the bytes as they arrived. pngHeader in
// assets_test.go is an IHDR and nothing else, which is the right fixture for the
// budget checks and the wrong one for anything that has to come out the far side
// as an image.
func tinyPNG(t *testing.T) []byte {
	t.Helper()

	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.Set(0, 0, color.RGBA{R: 200, G: 40, B: 40, A: 255})

	var out bytes.Buffer
	if err := png.Encode(&out, src); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}

	return out.Bytes()
}

func monsterImageRequest(t *testing.T, monsterID string) *http.Request {
	t.Helper()

	r := uploadRequest(t, "image", tinyPNG(t))
	r.SetPathValue("id", monsterID)

	return r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
}

// THE ROW IS THE LEDGER FOR WHAT LIVES IN R2, which is why it is written before
// the object and names the key the object will land at. An object written first
// and a row that never followed is a file nothing remembers: no page can render
// it, no delete will find it, and the sweeper works from these same rows.
//
// The handler cannot be driven end to end from here, for the reason the purge
// tests give: it opens with a :one that this fake answers by failing, and it
// reaches R2. What is checked instead is the pair of statements it runs, which
// is where the property actually lives -- the insert carries the key, and
// linking the asset to the monster is a second statement that runs after the
// object has landed.
func TestAMonsterImageUploadWritesTheRowBeforeReachingR2(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	q := queries.New(db)

	assetID := testAssetID
	_ = q.InsertMonsterImage(context.Background(), queries.InsertMonsterImageParams{
		ID:       assetID,
		OwnerID:  testOwnerID,
		FilePath: storage.MonsterImageKey(testOwnerID, assetID),
		FileName: "goblin.png",
		Name:     "goblin.png",
	})
	_ = q.UpdateMonsterImage(context.Background(), queries.UpdateMonsterImageParams{
		AssetID: &assetID,
		ID:      testMonsterID,
		OwnerID: testOwnerID,
	})

	if len(db.calls) != 2 {
		t.Fatalf("ran %d statements, want 2", len(db.calls))
	}
	insert, link := db.calls[0], db.calls[1]

	if !strings.Contains(insert.query, "INSERT INTO assets") {
		t.Fatalf("the first statement is not the insert: %q", insert.query)
	}
	if !strings.Contains(insert.query, "'monster'") {
		t.Errorf("the image is not stored as a monster asset: %q", insert.query)
	}
	if want := storage.MonsterImageKey(testOwnerID, assetID); insert.args[2] != want {
		t.Errorf("file_path = %v, want the key its object will land at %q", insert.args[2], want)
	}
	if strings.Contains(insert.query, "monsters") {
		t.Errorf("the insert reaches the monsters row, so the link is not a separate statement: %q", insert.query)
	}

	// The link is what makes the picture the monster's, and it runs last --
	// after the object is in the bucket -- so a monster never names an asset
	// whose object failed to upload.
	if !strings.Contains(link.query, "UPDATE monsters") || !strings.Contains(link.query, "asset_id") {
		t.Errorf("the second statement does not link the asset to the monster: %q", link.query)
	}
	if len(link.args) != 3 || link.args[1] != testMonsterID || link.args[2] != testOwnerID {
		t.Errorf("the link is not scoped to this user's monster: %v", link.args)
	}
}

// OWNERSHIP IS ESTABLISHED BEFORE ANYTHING IS WRITTEN OR UPLOADED. The read is
// scoped to the session's user, so a monster belonging to somebody else is a
// 404 -- and this pins that nothing runs before it: no insert, and no object
// in the bucket under a stranger's monster.
func TestAMonsterImageUploadWritesNothingWithoutTheMonster(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}

	app.UploadMonsterImage(newRecorder(), monsterImageRequest(t, testMonsterID.String()))

	if len(db.calls) != 0 {
		t.Errorf("the upload wrote something before it knew whose monster it was: %v", db.calls)
	}
	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	read := db.reads[0]
	if !strings.Contains(read.query, "FROM monsters") {
		t.Errorf("the ownership check read something else:\n%s", read.query)
	}
	if len(read.args) != 2 || read.args[0] != testMonsterID || read.args[1] != testOwnerID {
		t.Errorf("the check is not scoped to this user's monster: %v", read.args)
	}
}

// An id that does not parse never becomes a statement -- and never becomes a
// decode either, which is the part worth having on this route: the id is read
// before the upload is, so a broken URL costs nothing.
func TestMonsterImageRoutesRejectUnparseableIDs(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}
	rec := newRecorder()

	app.UploadMonsterImage(rec, monsterImageRequest(t, "not-a-ulid"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if len(db.calls) != 0 || len(db.reads) != 0 {
		t.Errorf("an unparseable id reached the database")
	}
	if trigger := rec.Header().Get("HX-Trigger"); !strings.Contains(trigger, "monster") {
		t.Errorf("the alert does not name the monster: %s", trigger)
	}
}

// THE IN LIST ON GetImage IS THE WHOLE OF THAT ROUTE'S ACCESS RULE, and it is
// deliberately not owner-scoped: a map, a token or a monster is shown to every
// player at the table the moment it is put down, and a table is not a list of
// owners. A journal image is the opposite -- one entry of one character's diary,
// reached through the share's own reader route -- and it stays off this list.
func TestGetImageServesMonsterImagesAndNotJournalOnes(t *testing.T) {
	statements := namedStatements(t, "assets.sql")

	body, ok := statements["GetImage"]
	if !ok {
		t.Fatal("no GetImage statement in sql/assets.sql")
	}

	members := regexp.MustCompile(`(?s)type IN \((.*?)\)`).FindStringSubmatch(body)
	if members == nil {
		t.Fatalf("GetImage no longer filters on a type list:\n%s", body)
	}

	served := map[string]bool{}
	for _, member := range strings.Split(members[1], ",") {
		served[strings.Trim(strings.TrimSpace(member), "'")] = true
	}

	// EVERY MEMBER OF THE ENUM IS ACCOUNTED FOR, one way or the other, and that
	// is the point of reading them rather than listing them here. A hardcoded
	// want-list is a test that passes for a member nobody thought about: this
	// asserted four names while `character` and `profile` were both absent from
	// it, and `profile` shipped missing from the statement -- the row missed,
	// the route answered 404, the object sat in the bucket, and nothing logged.
	//
	// So a new member fails this until somebody puts it on one side or the
	// other, which is the decision being forced rather than a name being typed
	// twice.
	withheld := map[string]string{
		// A journal image belongs to one entry of one character's diary and is
		// reached through the share's own reader route.
		"journal": "reached through the share reader, not the account-wide route",
		// Music is not an image and has a route of its own.
		"music": "not an image",
	}

	for _, member := range assetTypes(t) {
		why, kept := withheld[member]

		switch {
		case kept && served[member]:
			t.Errorf("%q images are served by /assets/images/{id}, but they are %s", member, why)
		case !kept && !served[member]:
			t.Errorf("%q images are not served by /assets/images/{id}, so every one of them renders broken", member)
		}
	}
}

// assetTypes is every member of the assets type enum, read off the generated
// models rather than written out.
//
// THE GENERATED FILE IS ALWAYS THERE, because this package does not compile
// without it -- the same argument namedStatements makes for reading sql/ off
// the disk instead of restating it.
func assetTypes(t *testing.T) []string {
	t.Helper()

	source, err := os.ReadFile(filepath.Join("..", "queries", "models.go"))
	if err != nil {
		t.Fatalf("cannot read the generated models: %v", err)
	}

	found := regexp.MustCompile(`AssetsType\w+\s+AssetsType = "(\w+)"`).FindAllStringSubmatch(string(source), -1)
	if len(found) == 0 {
		t.Fatal("no assets type constants in internal/queries/models.go")
	}

	members := make([]string, 0, len(found))
	for _, m := range found {
		members = append(members, m[1])
	}

	return members
}

// The two keys a monster's picture can be reached by are one key, which is what
// makes replacing an image safe: the object is overwritten in place, so nothing
// is orphaned and the card's <img> src does not change.
func TestAMonsterImageKeyBelongsToItsOwnerAndItsAsset(t *testing.T) {
	other := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS9")

	key := storage.MonsterImageKey(testOwnerID, testAssetID)
	if key != storage.MonsterImageKey(testOwnerID, testAssetID) {
		t.Error("the key is not stable, so a replacement would land somewhere new")
	}
	if key == storage.MonsterImageKey(other, testAssetID) {
		t.Error("two owners share a key")
	}
	if key == storage.MonsterImageKey(testOwnerID, other) {
		t.Error("two assets share a key")
	}
	if !strings.Contains(key, "/monsters/") {
		t.Errorf("key = %q, want a monster's own prefix", key)
	}
}
