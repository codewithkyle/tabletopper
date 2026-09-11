package controllers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
)

func pngPixels(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	return out.Bytes()
}
func libraryUploadRequest(t *testing.T, path string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("image", "rowboat.png")
	if err != nil {
		t.Fatalf("building the form: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("closing the form: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body.Bytes()))
	r.Header.Set("Content-Type", form.FormDataContentType())
	return r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
}
func TestAvatarsAreCroppedSquareAndTokensKeepTheirShape(t *testing.T) {
	for name, c := range map[string]struct {
		kind          libraryKind
		width, height int
		wantW, wantH  int
	}{
		"a wide avatar is cropped to a square":  {avatarKind, 300, 100, avatarLibrarySize, avatarLibrarySize},
		"a tall avatar is cropped to a square":  {avatarKind, 100, 300, avatarLibrarySize, avatarLibrarySize},
		"a small avatar is grown to the square": {avatarKind, 32, 32, avatarLibrarySize, avatarLibrarySize},
		"a wide token keeps its aspect":         {tokenKind, 1024, 256, tokenSize, tokenSize / 4},
		"a tall token keeps its aspect":         {tokenKind, 256, 1024, tokenSize / 4, tokenSize},
		"a square token fills the box":          {tokenKind, 1024, 1024, tokenSize, tokenSize},
		"a small token is left alone":           {tokenKind, 64, 21, 64, 21},
	} {
		t.Run(name, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, c.width, c.height))
			encoded, width, height, err := c.kind.encode(src)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if width != c.wantW || height != c.wantH {
				t.Errorf("stored %dx%d, want %dx%d", width, height, c.wantW, c.wantH)
			}
			if len(encoded) == 0 {
				t.Error("encode produced no bytes")
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("the encoded image does not decode: %v", err)
			}
			if cfg.Width != width || cfg.Height != height {
				t.Errorf("the row would say %dx%d for a file that is %dx%d", width, height, cfg.Width, cfg.Height)
			}
		})
	}
}
func TestUploadLibraryAssetWritesTheRowBeforeReachingR2(t *testing.T) {
	for _, kind := range []libraryKind{avatarKind, tokenKind} {
		t.Run(kind.Slug, func(t *testing.T) {
			db := &recordingDB{err: errNoRowsToGive}
			app := &App{Queries: queries.New(db)}
			r := libraryUploadRequest(t, "/assets/"+kind.Slug, pngPixels(t, 128, 128))
			app.uploadLibrary(newRecorder(), r, kind)
			if len(db.calls) != 1 {
				t.Fatalf("ran %d statements, want 1", len(db.calls))
			}
			insert := db.calls[0]
			if !strings.Contains(insert.query, "INSERT INTO assets") {
				t.Fatalf("the first statement is not the insert: %q", insert.query)
			}
			var found bool
			for _, arg := range insert.args {
				if bound, ok := arg.(queries.AssetsType); ok {
					found = true
					if bound != kind.Type {
						t.Errorf("the row is inserted as %q, want %q", bound, kind.Type)
					}
				}
			}
			if !found {
				t.Errorf("the insert binds no type at all: %v", insert.args)
			}
		})
	}
}
func TestLibraryReadsAndRenamesAreScopedToTheirKind(t *testing.T) {
	const id = "01BX5ZZKBKACTAV9WEVGEMMVS2"
	for _, kind := range []libraryKind{avatarKind, tokenKind} {
		t.Run(kind.Slug, func(t *testing.T) {
			db := &recordingDB{err: errNoRowsToGive}
			app := &App{Queries: queries.New(db)}
			r := httptest.NewRequest(http.MethodPatch, "/assets/"+kind.Slug+"/"+id+"/name", strings.NewReader("name=Rowboat"))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.SetPathValue("id", id)
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			app.renameLibrary(newRecorder(), r, kind.assetKind)
			if len(db.calls) != 1 {
				t.Fatalf("ran %d statements, want 1", len(db.calls))
			}
			rename := db.calls[0]
			if !strings.Contains(rename.query, "type = ?") {
				t.Errorf("the rename is not scoped to a kind: %q", rename.query)
			}
			var bound queries.AssetsType
			for _, arg := range rename.args {
				if v, ok := arg.(queries.AssetsType); ok {
					bound = v
				}
			}
			if bound != kind.Type {
				t.Errorf("the rename is scoped to %q, want %q", bound, kind.Type)
			}
		})
	}
}
func TestAssetNameIsCutToWhatTheColumnHolds(t *testing.T) {
	for name, c := range map[string]struct{ in, want string }{
		"short enough":        {"Rowboat", "Rowboat"},
		"exactly the limit":   {strings.Repeat("a", 255), strings.Repeat("a", 255)},
		"one over":            {strings.Repeat("a", 256), strings.Repeat("a", 255)},
		"accents are counted": {strings.Repeat("é", 300), strings.Repeat("é", 255)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := assetName(c.in); got != c.want {
				t.Errorf("assetName kept %d characters, want %d", len([]rune(got)), len([]rune(c.want)))
			}
		})
	}
}
func TestTheLibraryKindsShareNothing(t *testing.T) {
	if avatarKind.Type == tokenKind.Type {
		t.Error("both kinds insert the same assets.type")
	}
	if avatarKind.Slug == tokenKind.Slug {
		t.Error("both kinds route under the same segment")
	}
	if avatarKind.One == tokenKind.One {
		t.Error("both kinds call themselves the same thing")
	}
	if avatarKind.Key(testOwnerID, testOwnerID) == tokenKind.Key(testOwnerID, testOwnerID) {
		t.Error("both kinds store their objects at the same key")
	}
	if got := avatarKind.Key(testOwnerID, testOwnerID); got != storage.AvatarKey(testOwnerID, testOwnerID) {
		t.Errorf("the avatar kind stores at %q", got)
	}
	if got := tokenKind.Key(testOwnerID, testOwnerID); got != storage.TokenKey(testOwnerID, testOwnerID) {
		t.Errorf("the token kind stores at %q", got)
	}
}
func TestACharacterPortraitIsNotWrittenAsALibraryAvatar(t *testing.T) {
	db := &recordingDB{}
	q := queries.New(db)
	err := q.InsertCharacterPortrait(t.Context(), queries.InsertCharacterPortraitParams{
		ID:      testOwnerID,
		OwnerID: testOwnerID,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	insert := db.calls[0]
	if !strings.Contains(insert.query, "'character'") {
		t.Errorf("a portrait is not inserted as a character: %q", insert.query)
	}
	if strings.Contains(insert.query, "'avatar'") {
		t.Errorf("a portrait is still inserted as an avatar, which the library page would offer to delete: %q", insert.query)
	}
}
func TestTheImageRouteStillServesACharacterPortrait(t *testing.T) {
	db := &recordingDB{}
	q := queries.New(db)
	_, _ = q.GetImage(t.Context(), testOwnerID)
	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	if !strings.Contains(db.reads[0].query, "'character'") {
		t.Errorf("the image route does not serve a character portrait: %q", db.reads[0].query)
	}
}
func TestALibraryPageListsOneKindForOneOwner(t *testing.T) {
	for _, kind := range []libraryKind{avatarKind, tokenKind} {
		t.Run(kind.Slug, func(t *testing.T) {
			db := &recordingDB{err: errNoRowsToGive}
			app := &App{Queries: queries.New(db)}
			r := httptest.NewRequest(http.MethodGet, "/assets/"+kind.Slug, nil)
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			app.libraryPage(newRecorder(), r, kind)
			if len(db.calls) != 1 {
				t.Fatalf("ran %d statements, want 1", len(db.calls))
			}
			list := db.calls[0]
			if !strings.Contains(list.query, "owner_id = ?") || !strings.Contains(list.query, "type = ?") {
				t.Errorf("the listing is not scoped to an owner and a kind: %q", list.query)
			}
			var owner bool
			var bound queries.AssetsType
			for _, arg := range list.args {
				if id, ok := boundID(arg); ok && id == testOwnerID {
					owner = true
				}
				if v, ok := arg.(queries.AssetsType); ok {
					bound = v
				}
			}
			if !owner {
				t.Errorf("the listing is not scoped to the session's user: %v", list.args)
			}
			if bound != kind.Type {
				t.Errorf("the listing asks for %q, want %q", bound, kind.Type)
			}
		})
	}
}
