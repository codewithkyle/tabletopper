package controllers

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"hash/crc32"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
	"tabletopper/internal/tiling"

	"github.com/oklog/ulid/v2"
)

// pngHeader builds a PNG that is a signature and an IHDR chunk and nothing
// else: no pixel data, no IDAT, no IEND. It is a valid file to
// image.DecodeConfig, which reads the dimensions out of IHDR and stops, and
// garbage to any decoder that tries to produce an image from it.
//
// That is exactly the fixture the budget needs. A real 20,000 by 20,000 PNG
// would be the thing under test committed to the repository; this is fifty
// bytes and declares the same canvas.
//
// The CRC is computed rather than hard-coded because the chunk parser checks
// it, and colour type 6 -- 8-bit RGBA -- is chosen because DecodeConfig returns
// as soon as it has IHDR for anything that is not paletted.
func pngHeader(width, height uint32) []byte {
	chunk := []byte("IHDR")
	chunk = binary.BigEndian.AppendUint32(chunk, width)
	chunk = binary.BigEndian.AppendUint32(chunk, height)
	chunk = append(chunk, 8, 6, 0, 0, 0)

	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, uint32(len(chunk)-len("IHDR")))
	out = append(out, chunk...)

	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

// uploadRequest posts content as one multipart file under field.
func uploadRequest(t *testing.T, field string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile(field, "huge.png")
	if err != nil {
		t.Fatalf("building the form: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("closing the form: %v", err)
	}

	r := httptest.NewRequest(http.MethodPost, "/assets/maps", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())

	return r
}

// THE REFUSAL HAPPENS BEFORE THE DECODE, and the pair of cases is what proves
// it rather than asserting it.
//
// Both fixtures are a header and nothing else, so neither can be decoded into
// an image. The oversized one is answered 413 from its declared dimensions --
// which the decoder never got far enough to disagree with -- and the small one
// gets past the budget and fails afterwards with the 415 a corrupt file
// deserves. A check placed after image.Decode would have answered 415 for both,
// having first allocated 1.6 GB for the one it was supposed to refuse.
func TestReadImageUploadRefusesOverThePixelBudget(t *testing.T) {
	for _, c := range []struct {
		name          string
		width, height uint32
		status        int
	}{
		{name: "over the budget", width: 20000, height: 20000, status: http.StatusRequestEntityTooLarge},
		{name: "within the budget", width: 100, height: 100, status: http.StatusUnsupportedMediaType},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := uploadRequest(t, "map", pngHeader(c.width, c.height))

			if _, _, ok := readImageUpload(rec, r, "map"); ok {
				t.Fatal("readImageUpload accepted a file with no pixels in it")
			}
			if rec.Code != c.status {
				t.Errorf("status = %d, want %d", rec.Code, c.status)
			}
			// It answered the request itself, which is what ok=false promises.
			if !strings.Contains(rec.Header().Get("HX-Trigger"), "alert") {
				t.Errorf("no alert in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
			}
		})
	}
}

// THE MAP PATH DOES NOT DECODE, and this is the pair that proves it. The
// fixture is the same header-and-nothing-else PNG the test above feeds to
// readImageUpload, which answers 415 because there is nothing there to decode.
// readImageBytes accepts it, because it never asks: the header pass is the
// whole of the validation, and what comes back is the file as it arrived.
func TestReadImageBytesDoesNotDecode(t *testing.T) {
	fixture := pngHeader(100, 100)

	rec := httptest.NewRecorder()
	body, filename, contentType, ok := readImageBytes(rec, uploadRequest(t, "map", fixture), "map")

	if !ok {
		t.Fatalf("readImageBytes refused a valid header with status %d", rec.Code)
	}
	if !bytes.Equal(body, fixture) {
		t.Errorf("read %d bytes, want the %d that were uploaded", len(body), len(fixture))
	}
	if filename != "huge.png" {
		t.Errorf("filename = %q, want %q", filename, "huge.png")
	}
	// The multipart part was written as application/octet-stream, so this can
	// only have come from the header pass.
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want %q -- it must come from the header, not the browser", contentType, "image/png")
	}
}

// The budget is refused on the way through the same header pass, so the map
// path is no more willing to take a 400-megapixel canvas than the avatar path
// is -- it just refuses it without ever holding one.
func TestReadImageBytesRefusesOverThePixelBudget(t *testing.T) {
	rec := httptest.NewRecorder()

	if _, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", pngHeader(20000, 20000)), "map"); ok {
		t.Fatal("readImageBytes accepted a canvas over the budget")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

// THE ROW GOES BEFORE THE OBJECT, and the nil Storage on this App is what
// proves it: R2 is not reachable from here at all, so a handler that uploaded
// first would have panicked before any statement was recorded. What is asserted
// beyond the order is the shape of the row a map is now born with -- the
// original's key, the size to tile it at, and no preview, because there is
// nothing to preview until the worker has built something.
func TestUploadMapWritesTheRowBeforeReachingR2(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	app := &App{Queries: queries.New(db)}

	r := uploadRequest(t, "map", pngHeader(100, 100))
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()

	app.UploadMap(rec, r)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	insert := db.calls[0]
	if !strings.Contains(insert.query, "INSERT INTO assets") {
		t.Fatalf("the first statement is not the insert: %q", insert.query)
	}
	if !strings.Contains(insert.query, "'pending'") {
		t.Errorf("a map is not inserted pending, so nothing would ever tile it: %q", insert.query)
	}
	if strings.Contains(insert.query, "preview_path") {
		t.Errorf("the insert names a preview that does not exist yet: %q", insert.query)
	}

	if len(insert.args) != 6 {
		t.Fatalf("the insert takes %d arguments, want 6", len(insert.args))
	}
	assetID, ok := insert.args[0].(ulid.ULID)
	if !ok {
		t.Fatalf("the first argument is %T, want a ULID", insert.args[0])
	}
	if want := storage.MapOriginalKey(testOwnerID, assetID); insert.args[2] != want {
		t.Errorf("file_path = %v, want the original's key %q", insert.args[2], want)
	}
	if want := (sql.NullInt16{Int16: tiling.DefaultTileSize, Valid: true}); insert.args[5] != want {
		t.Errorf("tile_size = %v, want %v", insert.args[5], want)
	}
}

// The retry is owner-scoped and conditional on the row having failed, so a
// button pressed twice cannot re-queue a job that is already running and a
// stranger's id cannot re-queue anything at all.
func TestRetryMapTilingIsScopedAndConditional(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodPost, "/assets/maps/x/tiles", nil)
	r.SetPathValue("id", testAssetID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()

	app.RetryMapTiling(rec, r)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	retry := db.calls[0]
	for _, want := range []string{"owner_id = ?", "tile_state = 'failed'", "tile_attempts = 0"} {
		if !strings.Contains(retry.query, want) {
			t.Errorf("the retry does not contain %q: %q", want, retry.query)
		}
	}
	if len(retry.args) != 2 || retry.args[0] != testAssetID || retry.args[1] != testOwnerID {
		t.Errorf("the retry ran with %v, want the asset and the session's owner", retry.args)
	}
}

// An id that does not parse never becomes a statement, on either of the two
// map routes that take one and answer through the alert header.
func TestMapRoutesRejectUnparseableIDs(t *testing.T) {
	for name, handler := range map[string]func(*App) http.HandlerFunc{
		"retry":  func(a *App) http.HandlerFunc { return a.RetryMapTiling },
		"delete": func(a *App) http.HandlerFunc { return a.DeleteMap },
	} {
		t.Run(name, func(t *testing.T) {
			db := &recordingDB{}
			app := &App{Queries: queries.New(db)}

			r := httptest.NewRequest(http.MethodPost, "/assets/maps/x", nil)
			r.SetPathValue("id", "not-a-ulid")
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			rec := httptest.NewRecorder()

			handler(app)(rec, r)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if len(db.calls) != 0 {
				t.Errorf("ran %d statements, want 0", len(db.calls))
			}
		})
	}
}
