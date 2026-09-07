package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// uploadBody wraps content as one multipart file under field, and returns the
// bytes and the Content-Type that describes them.
func uploadBody(t *testing.T, field string, content []byte) ([]byte, string) {
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

	return body.Bytes(), form.FormDataContentType()
}

// uploadRequest posts content as one multipart file under field.
func uploadRequest(t *testing.T, field string, content []byte) *http.Request {
	t.Helper()

	body, contentType := uploadBody(t, field, content)
	r := httptest.NewRequest(http.MethodPost, "/assets/maps", bytes.NewReader(body))
	r.Header.Set("Content-Type", contentType)

	return r
}

// deadlineRecorder is a ResponseRecorder that answers the two methods
// http.ResponseController looks for. httptest's does not, so without this every
// upload test would run the path where extending the deadline failed -- which
// is the one path a real request must never take -- and would say nothing about
// whether it was extended.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	read  time.Time
	write time.Time
}

func (d *deadlineRecorder) SetReadDeadline(at time.Time) error  { d.read = at; return nil }
func (d *deadlineRecorder) SetWriteDeadline(at time.Time) error { d.write = at; return nil }

func newRecorder() *deadlineRecorder {
	return &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
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
			rec := newRecorder()
			r := uploadRequest(t, "map", pngHeader(c.width, c.height))

			if _, _, ok := readImageUpload(rec, r, "map", imageLimits); ok {
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

	rec := newRecorder()
	body, filename, contentType, ok := readImageBytes(rec, uploadRequest(t, "map", fixture), "map", mapLimits)

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

// AN UPLOAD LARGER THAN multipartMemory IS A FILE ON DISK, NOT BYTES IN A MAP,
// and that is the whole point of this test. ReadForm keeps the first 32 MiB of
// a part in memory and spills the rest to a temp file, so the same
// multipart.File is a rewindable byte slice below the line and an *os.File
// above it -- and only the second one notices being closed early.
//
// Which is what openImageUpload used to do: it deferred a Close and then
// returned the file it had just closed, and every caller closed it again. Under
// the line the extra Close is a no-op and nothing showed; over it, every map
// past 32 MiB failed on "file already closed" -- and the cap is 128.
//
// The fixture is a valid header followed by padding, because the header pass is
// the only part of a map that is ever decoded.
func TestAMapLargerThanTheMemoryBudgetIsStillReadableWhole(t *testing.T) {
	fixture := append(pngHeader(100, 100), make([]byte, multipartMemory+(1<<20))...)

	rec := newRecorder()
	body, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", fixture), "map", mapLimits)

	if !ok {
		t.Fatalf("readImageBytes refused a spilled upload with status %d", rec.Code)
	}
	if len(body) != len(fixture) {
		t.Errorf("read %d bytes of a %d byte upload", len(body), len(fixture))
	}
	if !bytes.Equal(body, fixture) {
		t.Error("the bytes that came back are not the ones that were sent")
	}
}

// The budget is refused on the way through the same header pass, so the map
// path is no more willing to take a 400-megapixel canvas than the avatar path
// is -- it just refuses it without ever holding one.
func TestReadImageBytesRefusesOverThePixelBudget(t *testing.T) {
	rec := newRecorder()

	if _, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", pngHeader(20000, 20000)), "map", mapLimits); ok {
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
	rec := newRecorder()

	app.UploadMap(rec, r)

	if len(db.calls) != 1 {
		t.Fatalf("ran %d statements, want 1", len(db.calls))
	}
	insert := db.calls[0]
	if !strings.Contains(insert.query, "INSERT INTO assets") {
		t.Fatalf("the first statement is not the insert: %q", insert.query)
	}
	if strings.Contains(insert.query, "tile_state") {
		t.Errorf("the insert queues the map before its original is uploaded: %q", insert.query)
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

// A map is queued for tiling by a statement of its own, and not by the insert.
//
// THIS IS THE RACE THAT REACHED PRODUCTION. The insert used to write
// tile_state = 'pending' itself, which made the row a job the moment it
// committed -- while the original it names was still going up to R2. The
// worker looks every five seconds, so an upload that took longer than the gap
// to the next pass got claimed with nothing to read, and the job died on a
// missing key. Pressing retry then worked every time, because by then the PUT
// had landed: the same row, the same key, the only difference being that the
// object had arrived.
//
// The two halves are pinned together here because neither is wrong alone. The
// insert may not write the value the claim looks for, and the claim must look
// for the value the second statement writes; a change to either that did not
// change the other would put the window back.
func TestAMapIsNotAJobUntilItsOriginalHasLanded(t *testing.T) {
	db := &recordingDB{err: errNoRowsToGive}
	q := queries.New(db)

	_ = q.InsertMap(context.Background(), queries.InsertMapParams{})
	_ = q.QueueMapForTiling(context.Background(), queries.QueueMapForTilingParams{})
	_, _ = q.ClaimMapForTiling(context.Background(), nil)

	if len(db.calls) != 3 {
		t.Fatalf("ran %d statements, want 3", len(db.calls))
	}
	insert, queue, claim := db.calls[0].query, db.calls[1].query, db.calls[2].query

	if strings.Contains(insert, "tile_state") {
		t.Errorf("the insert sets a tile state, so the row is claimable before the PUT: %q", insert)
	}
	if !strings.Contains(queue, "tile_state = 'pending'") {
		t.Errorf("the queue statement does not make the row pending: %q", queue)
	}
	if !strings.Contains(claim, "tile_state = 'pending'") {
		t.Errorf("the claim no longer matches on pending, so a fresh row may be claimable: %q", claim)
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

// AN UPLOAD LIFTS BOTH OF ITS OWN DEADLINES, and both is the point. The server
// sets five seconds to read and ten to write, and the write deadline runs from
// when the request's headers arrived rather than from when the handler answers
// -- so an upload that took a minute would read to completion and then fail to
// reply, which is the harder of the two to work out from the outside.
func TestAnUploadExtendsBothOfItsDeadlines(t *testing.T) {
	rec := newRecorder()
	before := time.Now()

	if _, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", pngHeader(100, 100)), "map", mapLimits); !ok {
		t.Fatal("the upload was refused")
	}

	if rec.read.Sub(before) < uploadReadDeadline {
		t.Errorf("the read deadline was extended by %v, want at least %v", rec.read.Sub(before), uploadReadDeadline)
	}
	if !rec.write.After(rec.read) {
		t.Errorf("the write deadline is %v and the read deadline %v; a request that uses its whole read budget could not answer", rec.write, rec.read)
	}
}

// THE MAP THIS WHOLE ARRANGEMENT EXISTS FOR IS 108 MEGAPIXELS, and the old
// single cap of 40 refused it outright. It is accepted now on the path that
// never decodes it, and still refused on the paths that do -- which is what
// keeps half a gigabyte of decoded avatar out of a request handler.
func TestTheMapCapTakesWhatTheDecodedCapCannot(t *testing.T) {
	const width, height = 12000, 9000

	rec := newRecorder()
	if _, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", pngHeader(width, height)), "map", mapLimits); !ok {
		t.Errorf("a %dx%d map was refused with status %d", width, height, rec.Code)
	}

	rec = newRecorder()
	if _, _, ok := readImageUpload(rec, uploadRequest(t, "map", pngHeader(width, height)), "map", imageLimits); ok {
		t.Errorf("a %dx%d image was accepted on the path that decodes it", width, height)
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}

	// Past the map's own cap it is refused there too, with a message that
	// names the cap it was given rather than one written into the string.
	rec = newRecorder()
	if _, _, _, ok := readImageBytes(rec, uploadRequest(t, "map", pngHeader(20000, 20000)), "map", mapLimits); ok {
		t.Error("a 400-megapixel map was accepted")
	}
	if alert := rec.Header().Get("HX-Trigger"); !strings.Contains(alert, "150 megapixels") {
		t.Errorf("the alert does not name the map cap: %q", alert)
	}
}

// http.MaxBytesReader is what enforces the byte cap, not the argument to
// ParseMultipartForm -- which is only how much is held in memory before the
// body spills to a temp file, and which is deliberately much smaller than the
// cap so that one upload is not a 128 MB heap allocation.
func TestTheByteCapIsEnforcedByMaxBytesReader(t *testing.T) {
	if multipartMemory >= maxMapBytes {
		t.Fatalf("multipartMemory is %d and the map cap %d; the buffer is meant to be the smaller of the two", multipartMemory, maxMapBytes)
	}

	rec := newRecorder()
	oversized := make([]byte, maxUploadBytes+(1<<20))

	if _, _, ok := readImageUpload(rec, uploadRequest(t, "avatar", oversized), "avatar", imageLimits); ok {
		t.Fatal("a file over the byte cap was accepted")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if alert := rec.Header().Get("HX-Trigger"); !strings.Contains(alert, "8 MiB") {
		t.Errorf("the alert does not name the cap it was given: %q", alert)
	}
}

// The globals this test's server runs with. They are a twentieth of the real
// ones so the test takes a second rather than half a minute; what is being
// shown is that the extension beats them, which does not depend on the numbers.
const (
	testReadTimeout  = 500 * time.Millisecond
	testWriteTimeout = 1 * time.Second
	testUploadSpan   = 2 * time.Second
)

// dribble runs handler on a real server with real timeouts and posts body to it
// slowly enough to exceed both, returning the raw response.
func dribble(t *testing.T, handler http.HandlerFunc, body []byte, contentType string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  testReadTimeout,
		WriteTimeout: testWriteTimeout,
	}
	go server.Serve(listener)
	defer server.Close()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "POST /assets/maps HTTP/1.1\r\nHost: test\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n", contentType, len(body))

	const chunks = 4
	go func() {
		size := (len(body) + chunks - 1) / chunks
		for start := 0; start < len(body); start += size {
			time.Sleep(testUploadSpan / chunks)
			if _, err := conn.Write(body[start:min(start+size, len(body))]); err != nil {
				return
			}
		}
	}()

	if err := conn.SetReadDeadline(time.Now().Add(testUploadSpan + 10*time.Second)); err != nil {
		t.Fatalf("setting the client deadline: %v", err)
	}
	// A read timeout on the server closes the connection, so the client sees a
	// reset rather than a reply. That is a failure to answer like any other
	// and is reported as one rather than ending the test.
	response, err := io.ReadAll(conn)
	if err != nil {
		return string(response) + "\n[the connection went away: " + err.Error() + "]"
	}
	return string(response)
}

// A REAL SERVER, ITS REAL GLOBAL TIMEOUTS, AND A CLIENT SLOWER THAN BOTH.
// Everything else here asserts against a recorder, which can only show that a
// deadline was asked for; this is what shows that asking works -- that a
// deadline set from inside the handler overrides the one the server had already
// established, for the body it has not finished reading and for the response it
// has not started writing.
//
// The control is the same server and the same slow client with a handler that
// does not ask. It has to fail, or the test proves nothing about the one that
// does.
func TestASlowUploadOutlivesTheServerTimeouts(t *testing.T) {
	fixture := pngHeader(100, 100)
	body, contentType := uploadBody(t, "map", fixture)

	t.Run("without extending", func(t *testing.T) {
		got := dribble(t, func(w http.ResponseWriter, r *http.Request) {
			if _, err := io.Copy(io.Discard, r.Body); err != nil {
				fmt.Fprint(w, "the body was cut off")
				return
			}
			fmt.Fprint(w, "read the whole body")
		}, body, contentType)

		if strings.Contains(got, "read the whole body") {
			t.Errorf("the server's read timeout did not bite, so this proves nothing: %q", got)
		}
	})

	t.Run("through the upload path", func(t *testing.T) {
		got := dribble(t, func(w http.ResponseWriter, r *http.Request) {
			read, _, _, ok := readImageBytes(w, r, "map", mapLimits)
			if !ok {
				return
			}
			fmt.Fprintf(w, "read %d bytes", len(read))
		}, body, contentType)

		if want := fmt.Sprintf("read %d bytes", len(fixture)); !strings.Contains(got, want) {
			t.Errorf("the upload did not survive the server's timeouts: %q", got)
		}
	})
}

// The card fragment is one map, read as its owner. It is behind auth.Fragment
// so there is a session to scope it to, and scoping it is what stops the poll
// URL from being a way to read the name and file name of any map by id -- which
// the tile route deliberately is, for a reason that does not apply here.
func TestTheCardFragmentReadsTheOwnersOwnMap(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodGet, "/fragment/assets/maps/x/card", nil)
	r.SetPathValue("id", testAssetID.String())
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
	rec := httptest.NewRecorder()

	app.MapCardFragment(rec, r)

	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	read := db.reads[0]
	if !strings.Contains(read.query, "owner_id = ?") {
		t.Errorf("the card fragment is not scoped to its owner: %q", read.query)
	}
	if len(read.args) != 2 || read.args[0] != testAssetID || read.args[1] != testOwnerID {
		t.Errorf("the read ran with %v, want the asset and the session's owner", read.args)
	}
}

// A POLL THAT FAILS SAYS NOTHING. The request was made by a timer rather than
// by the owner, so an alert dialog opening over the asset manager would be the
// page reporting on housekeeping nobody asked about. htmx leaves the target
// alone on a 4xx, so the card already on screen simply stays as it is.
func TestAFailedCardPollIsSilent(t *testing.T) {
	for name, id := range map[string]string{
		"a map that is gone":    testAssetID.String(),
		"an id that is not one": "not-a-ulid",
	} {
		t.Run(name, func(t *testing.T) {
			db := &recordingDB{}
			app := &App{Queries: queries.New(db)}

			r := httptest.NewRequest(http.MethodGet, "/fragment/assets/maps/x/card", nil)
			r.SetPathValue("id", id)
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			rec := httptest.NewRecorder()

			app.MapCardFragment(rec, r)

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if body := rec.Body.String(); body != "" {
				t.Errorf("body = %q, want nothing at all", body)
			}
			if trigger := rec.Header().Get("HX-Trigger"); trigger != "" {
				t.Errorf("HX-Trigger = %q, so a background poll opened a dialog", trigger)
			}
		})
	}
}

// An id that does not parse is answered before anything is queried.
func TestABadIDOnTheCardFragmentNeverBecomesAStatement(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}

	r := httptest.NewRequest(http.MethodGet, "/fragment/assets/maps/x/card", nil)
	r.SetPathValue("id", "not-a-ulid")
	r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))

	app.MapCardFragment(httptest.NewRecorder(), r)

	if len(db.reads) != 0 {
		t.Errorf("ran %d reads, want 0", len(db.reads))
	}
}
