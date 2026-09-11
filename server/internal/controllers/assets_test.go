package controllers

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
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
func uploadRequest(t *testing.T, field string, content []byte) *http.Request {
	t.Helper()
	body, contentType := uploadBody(t, field, content)
	r := httptest.NewRequest(http.MethodPost, "/assets/maps", bytes.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	return r
}

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
			if !strings.Contains(rec.Header().Get("HX-Trigger"), "alert") {
				t.Errorf("no alert in HX-Trigger: %q", rec.Header().Get("HX-Trigger"))
			}
		})
	}
}
func openMap(t *testing.T, rec http.ResponseWriter, r *http.Request) ([]byte, *multipart.FileHeader, string, bool) {
	t.Helper()
	file, header, contentType, ok := openImageUpload(rec, r, "map", mapLimits)
	if !ok {
		return nil, nil, "", false
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("reading the opened upload: %v", err)
	}
	return body, header, contentType, true
}
func TestTheMapPathDoesNotDecode(t *testing.T) {
	fixture := pngHeader(100, 100)
	rec := newRecorder()
	body, header, contentType, ok := openMap(t, rec, uploadRequest(t, "map", fixture))
	if !ok {
		t.Fatalf("the map path refused a valid header with status %d", rec.Code)
	}
	if !bytes.Equal(body, fixture) {
		t.Errorf("read %d bytes, want the %d that were uploaded", len(body), len(fixture))
	}
	if header.Filename != "huge.png" {
		t.Errorf("filename = %q, want %q", header.Filename, "huge.png")
	}
	if header.Size != int64(len(fixture)) {
		t.Errorf("header.Size = %d, want %d -- it is the ContentLength the PUT is given", header.Size, len(fixture))
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want %q -- it must come from the header, not the browser", contentType, "image/png")
	}
}
func TestAMapLargerThanTheMemoryBudgetIsStillReadableWhole(t *testing.T) {
	fixture := append(pngHeader(100, 100), make([]byte, multipartMemory+(1<<20))...)
	rec := newRecorder()
	body, header, _, ok := openMap(t, rec, uploadRequest(t, "map", fixture))
	if !ok {
		t.Fatalf("the map path refused a spilled upload with status %d", rec.Code)
	}
	if len(body) != len(fixture) {
		t.Errorf("read %d bytes of a %d byte upload", len(body), len(fixture))
	}
	if !bytes.Equal(body, fixture) {
		t.Error("the bytes that came back are not the ones that were sent")
	}
	if header.Size != int64(len(fixture)) {
		t.Errorf("header.Size = %d, want %d", header.Size, len(fixture))
	}
}
func TestAnOpenedMapIsSeekableForThePut(t *testing.T) {
	for name, fixture := range map[string][]byte{
		"in memory": pngHeader(100, 100),
		"spilled":   append(pngHeader(100, 100), make([]byte, multipartMemory+(1<<20))...),
	} {
		t.Run(name, func(t *testing.T) {
			file, _, _, ok := openImageUpload(newRecorder(), uploadRequest(t, "map", fixture), "map", mapLimits)
			if !ok {
				t.Fatal("the map path refused the upload")
			}
			defer file.Close()
			var seeker io.ReadSeeker = file
			if _, err := seeker.Seek(0, io.SeekEnd); err != nil {
				t.Fatalf("seeking to the end: %v", err)
			}
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				t.Fatalf("seeking back to the start: %v", err)
			}
			body, err := io.ReadAll(seeker)
			if err != nil {
				t.Fatalf("reading after the rewind: %v", err)
			}
			if !bytes.Equal(body, fixture) {
				t.Errorf("read %d bytes after a rewind, want the %d that were sent", len(body), len(fixture))
			}
		})
	}
}
func TestTheMapPathRefusesOverThePixelBudget(t *testing.T) {
	rec := newRecorder()
	if _, _, _, ok := openMap(t, rec, uploadRequest(t, "map", pngHeader(20000, 20000))); ok {
		t.Fatal("the map path accepted a canvas over the budget")
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
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
	if len(insert.args) != 7 {
		t.Fatalf("the insert takes %d arguments, want 7", len(insert.args))
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
	if want := int64(len(pngHeader(100, 100))); insert.args[6] != want {
		t.Errorf("size_bytes = %v, want the original's %d bytes", insert.args[6], want)
	}
}
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
func TestAnUploadExtendsBothOfItsDeadlines(t *testing.T) {
	rec := newRecorder()
	before := time.Now()
	if _, _, _, ok := openMap(t, rec, uploadRequest(t, "map", pngHeader(100, 100))); !ok {
		t.Fatal("the upload was refused")
	}
	if rec.read.Sub(before) < uploadReadDeadline {
		t.Errorf("the read deadline was extended by %v, want at least %v", rec.read.Sub(before), uploadReadDeadline)
	}
	if !rec.write.After(rec.read) {
		t.Errorf("the write deadline is %v and the read deadline %v; a request that uses its whole read budget could not answer", rec.write, rec.read)
	}
}
func TestTheMapCapTakesWhatTheDecodedCapCannot(t *testing.T) {
	const width, height = 12000, 9000
	rec := newRecorder()
	if _, _, _, ok := openMap(t, rec, uploadRequest(t, "map", pngHeader(width, height))); !ok {
		t.Errorf("a %dx%d map was refused with status %d", width, height, rec.Code)
	}
	rec = newRecorder()
	if _, _, ok := readImageUpload(rec, uploadRequest(t, "map", pngHeader(width, height)), "map", imageLimits); ok {
		t.Errorf("a %dx%d image was accepted on the path that decodes it", width, height)
	}
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	rec = newRecorder()
	if _, _, _, ok := openMap(t, rec, uploadRequest(t, "map", pngHeader(20000, 20000))); ok {
		t.Error("a 400-megapixel map was accepted")
	}
	if alert := rec.Header().Get("HX-Trigger"); !strings.Contains(alert, "150 megapixels") {
		t.Errorf("the alert does not name the map cap: %q", alert)
	}
}
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

const (
	testReadTimeout  = 500 * time.Millisecond
	testWriteTimeout = 1 * time.Second
	testUploadSpan   = 2 * time.Second
)

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
	response, err := io.ReadAll(conn)
	if err != nil {
		return string(response) + "\n[the connection went away: " + err.Error() + "]"
	}
	return string(response)
}
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
			read, _, _, ok := openMap(t, w, r)
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
func imageRequest(t *testing.T, preview bool, assetType string, previewPath driver.Value, etag string) *httptest.ResponseRecorder {
	t.Helper()
	updated := time.Unix(1_700_000_000, 0)
	stub := oneRowDB{
		columns: []string{"id", "type", "file_path", "preview_path", "updated_at"},
		values: []driver.Value{
			testAssetID.String(),
			[]byte(assetType),
			"users/x/maps/y/original.png",
			previewPath,
			updated,
		},
	}
	app := &App{Queries: queries.New(stub.db())}
	r := httptest.NewRequest(http.MethodGet, "/assets/images/"+testAssetID.String(), nil)
	r.SetPathValue("id", testAssetID.String())
	if etag != "" {
		r.Header.Set("If-None-Match", etag)
	}
	rec := httptest.NewRecorder()
	if preview {
		app.GetImagePreview(rec, r)
	} else {
		app.GetImage(rec, r)
	}
	return rec
}
func currentETag() string {
	return fmt.Sprintf(`"%s-%d"`, testAssetID, time.Unix(1_700_000_000, 0).Unix())
}
func TestAMapWithNoPreviewHasNoImageAtEitherURL(t *testing.T) {
	for name, preview := range map[string]bool{"bare": false, "preview": true} {
		t.Run(name, func(t *testing.T) {
			rec := imageRequest(t, preview, "map", nil, currentETag())
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}
func TestAMapServesItsPreviewAndNeverItsOriginal(t *testing.T) {
	if got := imageRequest(t, false, "map", "users/x/maps/y/gen/preview.webp", currentETag()).Code; got != http.StatusNotFound {
		t.Errorf("the bare URL answered %d for a tiled map, want %d", got, http.StatusNotFound)
	}
	if got := imageRequest(t, true, "map", "users/x/maps/y/gen/preview.webp", currentETag()).Code; got != http.StatusNotModified {
		t.Errorf("the preview URL answered %d, want %d -- it did not reach the stream", got, http.StatusNotModified)
	}
}
func TestTheOtherImageTypesStillServeTheirFilePath(t *testing.T) {
	for _, assetType := range []string{"avatar", "token", "monster", "character"} {
		t.Run(assetType, func(t *testing.T) {
			if got := imageRequest(t, false, assetType, nil, currentETag()).Code; got != http.StatusNotModified {
				t.Errorf("the bare URL answered %d, want %d", got, http.StatusNotModified)
			}
			if got := imageRequest(t, true, assetType, nil, currentETag()).Code; got != http.StatusNotModified {
				t.Errorf("the preview URL answered %d, want %d", got, http.StatusNotModified)
			}
		})
	}
}
func TestADecodeWaitsForASlotAndThenGivesUp(t *testing.T) {
	for range cap(decodeSlots) {
		decodeSlots <- struct{}{}
	}
	defer func() {
		for range cap(decodeSlots) {
			<-decodeSlots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := decodeUpload(ctx, bytes.NewReader(pngHeader(10, 10))); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("decodeUpload on a full gate returned %v, want %v", err, context.DeadlineExceeded)
	}
}
func TestADecodeWithASlotFreeReachesTheDecoder(t *testing.T) {
	_, err := decodeUpload(context.Background(), bytes.NewReader(pngHeader(10, 10)))
	if err == nil {
		t.Fatal("a PNG header with no IDAT decoded")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("decodeUpload returned %v, want the decoder's error", err)
	}
}
func TestAvatarAndMonsterUploadsCheckOwnershipBeforeReadingTheBody(t *testing.T) {
	for name, c := range map[string]struct {
		handler func(*App) http.HandlerFunc
		table   string
	}{
		"avatar":  {func(a *App) http.HandlerFunc { return a.UploadCharacterAvatar }, "FROM characters"},
		"monster": {func(a *App) http.HandlerFunc { return a.UploadMonsterImage }, "FROM monsters"},
	} {
		t.Run(name, func(t *testing.T) {
			db := &recordingDB{err: errNoRowsToGive}
			app := &App{Queries: queries.New(db)}
			r := httptest.NewRequest(http.MethodPost, "/upload", nil)
			r.SetPathValue("id", testCharacterID.String())
			r = r.WithContext(session.NewContext(r.Context(), session.UserSession{UserID: testOwnerID}))
			rec := httptest.NewRecorder()
			c.handler(app)(rec, r)
			if len(db.reads) != 1 {
				t.Fatalf("ran %d reads before the body, want 1 -- the ownership check moved back after the decode", len(db.reads))
			}
			if !strings.Contains(db.reads[0].query, c.table) {
				t.Errorf("the first statement is %q, want the owner-scoped lookup on %s", db.reads[0].query, c.table)
			}
			if !strings.Contains(db.reads[0].query, "owner_id = ?") {
				t.Errorf("the first statement is not owner-scoped: %q", db.reads[0].query)
			}
		})
	}
}
