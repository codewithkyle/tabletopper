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

// pngPixels builds a real, decodable PNG of the given size. It is not
// pngHeader: that fixture is a header and nothing else, which is exactly right
// for the map path, because a map is never decoded in a request. Every library
// upload IS decoded and resized, so it needs pixels to resize.
func pngPixels(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	// One opaque pixel in the corner, so the encoder has something to do and
	// the decode is not trivially of an empty image.
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}

	return out.Bytes()
}

// libraryUploadRequest posts one picture as the "image" field every library
// route reads.
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

// A TOKEN KEEPS ITS SHAPE AND AN AVATAR DOES NOT, and this is the one thing the
// two kinds actually disagree about.
//
// An avatar is centre-cropped square, because a face is in the middle of the
// picture and a portrait is a thumbnail that sits in a fixed round hole. A
// token is fitted, because it is a longboat or a wagon drawn on a map at the
// shape somebody drew it -- and Square would store a square of hull.
//
// THE LAST CASE IS THE ONE THAT IS EASY TO GET WRONG. imaging.Fit scales down
// only, so a token smaller than the box is stored at its own size rather than
// blown up to 512 and stored blurry. A Resize would have enlarged it.
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

			// The dimensions reported are the STORED image's, so they have to
			// agree with what a decoder finds in the bytes -- otherwise a
			// renderer placing a token from the row would draw it at the shape
			// of the upload rather than of the file it fetches.
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

// THE ROW GOES BEFORE THE OBJECT, and the nil Storage on this App is what
// proves it: R2 is not reachable from here at all, so a handler that uploaded
// first would have panicked before any statement was recorded.
//
// The row is the ledger for what lives in R2. An object written under a key no
// row names is a file nothing can render, no delete will find and no sweep will
// collect, because every sweep works from these rows.
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

			// The type is bound, not defaulted. The column defaults to 'map',
			// so a statement that left it out would file every token and every
			// avatar in the map library.
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

// EVERY STATEMENT BEHIND A LIBRARY ROUTE CARRIES THE KIND, which is what makes
// the kind in the path mean something. Both of these are reached with an id and
// nothing else, and the row is the owner's either way -- so without the type in
// the WHERE, a token's id sent to DELETE /assets/avatars/{id} would delete the
// token and answer 200.
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

// A name is cut to what the column takes rather than refused, because nothing
// that can reach this has anywhere to show an error: the box carries maxlength,
// so an over-long name was composed by something that is not a browser, and a
// filename is not typed at all. MySQL runs strict, so the alternative to cutting
// is a driver error -- a 500 on a keystroke.
//
// The count is in runes because VARCHAR counts characters. A byte count would
// refuse names of accented characters that the column would have taken.
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

// The two descriptors have to disagree about everything they name, because one
// set of handlers reads all of it. A Key copied from the other kind would file
// every token under the avatars prefix, where nothing would be lost but nothing
// would be findable either.
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

	// And each key builder is the one its name says, so a rename of either
	// cannot quietly repoint the other.
	if got := avatarKind.Key(testOwnerID, testOwnerID); got != storage.AvatarKey(testOwnerID, testOwnerID) {
		t.Errorf("the avatar kind stores at %q", got)
	}
	if got := tokenKind.Key(testOwnerID, testOwnerID); got != storage.TokenKey(testOwnerID, testOwnerID) {
		t.Errorf("the token kind stores at %q", got)
	}
}

// A CHARACTER'S PORTRAIT IS NOT A LIBRARY AVATAR, and the statement is where
// that is decided. Both are one square picture on an assets row, so nothing
// downstream can tell them apart -- the type is the whole distinction.
//
// THE HAZARD IS DELETION AND IT IS SILENT. There is no foreign key anywhere in
// this schema; characters.asset_id is a plain nullable column. A portrait
// written as `avatar` would be listed by the Avatars page, which offers a
// Delete on every row it shows, and deleting it would leave the character
// pointing at nothing. The image route would answer 404 and the only symptom
// would be a portrait that had quietly become a broken picture.
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

// The image proxy has to keep serving what it served the day before the
// portrait moved. This is the other half of the rename: a type the insert
// writes and the read does not accept is a picture that uploads fine and then
// 404s on every page that shows it.
func TestTheImageRouteStillServesACharacterPortrait(t *testing.T) {
	db := &recordingDB{}
	q := queries.New(db)

	// The read fails -- recordingDB has no rows -- but the statement is
	// recorded first, which is what this is about.
	_, _ = q.GetImage(t.Context(), testOwnerID)

	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	if !strings.Contains(db.reads[0].query, "'character'") {
		t.Errorf("the image route does not serve a character portrait: %q", db.reads[0].query)
	}
}

// A library page lists one kind, scoped to the account looking at it. Both
// halves matter: without the owner it is everybody's shelf, and without the
// type it is every kind on one page -- including the portraits that were moved
// out of `avatar` precisely so they would not appear here.
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
