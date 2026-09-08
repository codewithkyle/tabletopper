package tiling

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"

	"github.com/chai2010/webp"
	"github.com/oklog/ulid/v2"
)

var (
	testOwner    = ulid.MustParse("01JAAAAAAAAAAAAAAAAAAAAAA1")
	testAsset    = ulid.MustParse("01JBBBBBBBBBBBBBBBBBBBBBB2")
	testLease    = ulid.MustParse("01JCCCCCCCCCCCCCCCCCCCCCC3")
	testPrevious = ulid.MustParse("01JDDDDDDDDDDDDDDDDDDDDDD4")
)

// MOST OF WHAT THIS PACKAGE HAS TO GET RIGHT IS AN ORDER -- which prefix is
// deleted before which column is written, and what is left alone when one of
// those fails -- so both fakes write into one journal and the order is what the
// tests read.
type journal struct {
	mu    sync.Mutex
	steps []string
}

func (j *journal) note(step string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.steps = append(j.steps, step)
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.steps...)
}

func (j *journal) indexOf(t *testing.T, step string) int {
	t.Helper()
	for i, got := range j.all() {
		if got == step {
			return i
		}
	}
	t.Fatalf("%q never happened; the journal is %v", step, j.all())
	return -1
}

type fakeResult struct {
	rows int64
	err  error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, errors.New("not used") }
func (r fakeResult) RowsAffected() (int64, error) { return r.rows, r.err }

type fakeBucket struct {
	log *journal

	mu      sync.Mutex
	stored  map[string][]byte
	deleted []string

	inFlight int
	peak     int

	original    []byte
	getErr      error
	putErrOn    string
	deleteErrOn string
}

func (b *fakeBucket) Get(context.Context, string) (io.ReadCloser, int64, error) {
	if b.getErr != nil {
		return nil, 0, b.getErr
	}
	return io.NopCloser(bytes.NewReader(b.original)), int64(len(b.original)), nil
}

func (b *fakeBucket) Put(_ context.Context, key string, body []byte, _ string) error {
	b.mu.Lock()
	b.inFlight++
	b.peak = max(b.peak, b.inFlight)
	b.mu.Unlock()

	// Long enough that tiles genuinely overlap, so the peak below is a
	// measurement rather than an artefact of how fast the encoder is.
	time.Sleep(time.Millisecond)

	b.mu.Lock()
	defer b.mu.Unlock()
	b.inFlight--

	if b.putErrOn != "" && strings.Contains(key, b.putErrOn) {
		return errors.New("the bucket refused " + key)
	}
	if b.stored == nil {
		b.stored = map[string][]byte{}
	}
	b.stored[key] = body
	return nil
}

func (b *fakeBucket) DeletePrefix(_ context.Context, prefix string) error {
	if b.deleteErrOn != "" && strings.Contains(prefix, b.deleteErrOn) {
		return errors.New("the bucket refused to delete " + prefix)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.deleted = append(b.deleted, prefix)
	b.log.note("delete " + prefix)
	for key := range b.stored {
		if strings.HasPrefix(key, prefix) {
			delete(b.stored, key)
		}
	}
	return nil
}

func (b *fakeBucket) keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.stored))
	for key := range b.stored {
		out = append(out, key)
	}
	return out
}

type fakeRows struct {
	log *journal

	asset    queries.Asset
	claimed  int64
	claimErr error
	getErr   error

	publishedRows int64
	publishErr    error

	stranded  []queries.ListStrandedTilingJobsRow
	strandErr error

	completed []queries.CompleteMapTilingParams
	failed    []queries.FailMapTilingParams
	requeued  []queries.RequeueStrandedTilingJobParams
	retried   []queries.RequeueFailedTilingJobsParams
}

func (r *fakeRows) ClaimMapForTiling(context.Context, *ulid.ULID) (sql.Result, error) {
	if r.claimErr != nil {
		return nil, r.claimErr
	}
	taken := r.claimed
	r.claimed = 0 // one map to claim, then the queue is empty
	return fakeResult{rows: taken}, nil
}

func (r *fakeRows) GetLeasedMap(_ context.Context, lease *ulid.ULID) (queries.Asset, error) {
	if r.getErr != nil {
		return queries.Asset{}, r.getErr
	}
	asset := r.asset
	asset.TileLease = lease
	return asset, nil
}

func (r *fakeRows) CompleteMapTiling(_ context.Context, arg queries.CompleteMapTilingParams) (sql.Result, error) {
	r.completed = append(r.completed, arg)
	r.log.note("publish")
	if r.publishErr != nil {
		return nil, r.publishErr
	}
	return fakeResult{rows: r.publishedRows}, nil
}

func (r *fakeRows) FailMapTiling(_ context.Context, arg queries.FailMapTilingParams) (sql.Result, error) {
	r.failed = append(r.failed, arg)
	r.log.note("fail")
	return fakeResult{rows: 1}, nil
}

func (r *fakeRows) ListStrandedTilingJobs(context.Context, sql.NullTime) ([]queries.ListStrandedTilingJobsRow, error) {
	return r.stranded, r.strandErr
}

func (r *fakeRows) RequeueStrandedTilingJob(_ context.Context, arg queries.RequeueStrandedTilingJobParams) (sql.Result, error) {
	r.requeued = append(r.requeued, arg)
	r.log.note("requeue")
	return fakeResult{rows: 1}, nil
}

func (r *fakeRows) RequeueFailedTilingJobs(_ context.Context, arg queries.RequeueFailedTilingJobsParams) (sql.Result, error) {
	r.retried = append(r.retried, arg)
	r.log.note("retry")
	return fakeResult{rows: 1}, nil
}

// A 20x12 source at a tile size of 8 is a three-level pyramid of nine tiles:
// 3x2 at level 0, 2x1 at level 1 and the single 5x3 tile that is level 2.
const (
	sourceWidth  = 20
	sourceHeight = 12
	sourceTile   = 8
	sourceTiles  = 9
	sourceZoom   = 2
)

func sourcePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, sourceWidth, sourceHeight))
	for y := 0; y < sourceHeight; y++ {
		for x := 0; x < sourceWidth; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 12), G: uint8(y * 20), B: 40, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encoding the test source: %v", err)
	}
	return out.Bytes()
}

// newWorker builds a worker over a bucket holding one map's original, and the
// asset row that names it.
func newWorker(t *testing.T) (*worker, *fakeRows, *fakeBucket, *journal, queries.Asset) {
	t.Helper()

	log := &journal{}
	bucket := &fakeBucket{log: log, original: sourcePNG(t)}
	asset := queries.Asset{
		ID:       testAsset,
		OwnerID:  testOwner,
		FilePath: storage.MapOriginalKey(testOwner, testAsset),
		Type:     queries.AssetsTypeMap,
		TileSize: sql.NullInt16{Int16: sourceTile, Valid: true},
		TileState: queries.NullAssetsTileState{
			AssetsTileState: queries.AssetsTileStateWorking, Valid: true,
		},
		TileLease: &testLease,
	}
	db := &fakeRows{log: log, asset: asset, publishedRows: 1}

	return &worker{db: db, bucket: bucket}, db, bucket, log, asset
}

// The whole pyramid lands under the generation's prefix, the preview beside it,
// and the row is published with the dimensions of what was actually built.
func TestTilingOneMapWritesTheWholeGeneration(t *testing.T) {
	w, db, bucket, _, asset := newWorker(t)

	w.tile(context.Background(), asset)

	generation := storage.MapGenerationPrefix(testOwner, testAsset, testLease)
	preview := storage.MapPreviewKey(testOwner, testAsset, testLease)

	tiles := 0
	for _, key := range bucket.keys() {
		if !strings.HasPrefix(key, generation) {
			t.Errorf("wrote %q, which is outside the generation being built", key)
		}
		if key != preview {
			tiles++
		}
	}
	if tiles != sourceTiles {
		t.Errorf("wrote %d tiles, want %d", tiles, sourceTiles)
	}
	for _, key := range []string{
		storage.MapTileKey(testOwner, testAsset, testLease, 0, 0, 0),
		storage.MapTileKey(testOwner, testAsset, testLease, 0, 2, 1),
		storage.MapTileKey(testOwner, testAsset, testLease, sourceZoom, 0, 0),
		preview,
	} {
		if _, ok := bucket.stored[key]; !ok {
			t.Errorf("%q was not written", key)
		}
	}

	if len(db.completed) != 1 {
		t.Fatalf("published %d times, want once", len(db.completed))
	}
	published := db.completed[0]
	if published.Width.Int32 != sourceWidth || published.Height.Int32 != sourceHeight {
		t.Errorf("published %dx%d, want %dx%d", published.Width.Int32, published.Height.Int32, sourceWidth, sourceHeight)
	}
	if published.MaxZoom.Int16 != sourceZoom {
		t.Errorf("published max zoom %d, want %d", published.MaxZoom.Int16, sourceZoom)
	}
	if published.TileSize.Int16 != sourceTile {
		t.Errorf("published tile size %d, want %d", published.TileSize.Int16, sourceTile)
	}
	if published.TileGen == nil || *published.TileGen != testLease {
		t.Errorf("published generation %v, want the lease %v", published.TileGen, testLease)
	}
	if published.TileLease == nil || *published.TileLease != testLease {
		t.Error("the publish does not name the lease it is completing, so it could land on another claim")
	}
	if published.PreviewPath.String != preview {
		t.Errorf("published preview %q, want %q", published.PreviewPath.String, preview)
	}
}

// The preview is built from the top of the pyramid rather than from the source,
// and it is a square of the size the map card renders.
func TestThePreviewIsASquareOfTheTopLevel(t *testing.T) {
	w, _, bucket, _, asset := newWorker(t)

	w.tile(context.Background(), asset)

	encoded, ok := bucket.stored[storage.MapPreviewKey(testOwner, testAsset, testLease)]
	if !ok {
		t.Fatal("no preview was written")
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("the preview is not a WebP: %v", err)
	}
	if cfg.Width != 256 || cfg.Height != 256 {
		t.Errorf("the preview is %dx%d, want 256x256", cfg.Width, cfg.Height)
	}
}

// The generation that was serving is deleted only after the row has been moved
// onto the new one. The other order would take the tiles out from under
// everyone still looking at the map.
func TestTheSupersededGenerationGoesAfterThePublish(t *testing.T) {
	w, _, bucket, log, asset := newWorker(t)
	asset.TileGen = &testPrevious

	w.tile(context.Background(), asset)

	superseded := storage.MapGenerationPrefix(testOwner, testAsset, testPrevious)
	if log.indexOf(t, "publish") > log.indexOf(t, "delete "+superseded) {
		t.Errorf("the old generation was deleted before the row moved off it: %v", log.all())
	}
	if len(bucket.deleted) != 1 || bucket.deleted[0] != superseded {
		t.Errorf("deleted %v, want only the superseded generation", bucket.deleted)
	}
}

// A first pyramid supersedes nothing, so nothing is deleted. A DeletePrefix on
// a generation that was never minted would be harmless and is still wrong: it
// is a list of the bucket for every map anyone ever uploads.
func TestAFirstPyramidDeletesNothing(t *testing.T) {
	w, _, bucket, _, asset := newWorker(t)

	w.tile(context.Background(), asset)

	if len(bucket.deleted) != 0 {
		t.Errorf("deleted %v, want nothing", bucket.deleted)
	}
}

// A REPLACEMENT ARRIVING MID-BUILD IS THE CASE THE LEASE EXISTS FOR. The
// conditional publish matches no row, which says the claim was taken away and
// the original this pyramid was built from has already been overwritten. What
// was just built goes; what is still serving is left exactly alone.
func TestLosingTheClaimDiscardsTheNewPyramidAndKeepsTheOld(t *testing.T) {
	w, db, bucket, _, asset := newWorker(t)
	asset.TileGen = &testPrevious
	db.publishedRows = 0

	w.tile(context.Background(), asset)

	built := storage.MapGenerationPrefix(testOwner, testAsset, testLease)
	superseded := storage.MapGenerationPrefix(testOwner, testAsset, testPrevious)

	if len(bucket.deleted) != 1 || bucket.deleted[0] != built {
		t.Errorf("deleted %v, want only the generation that was just built", bucket.deleted)
	}
	for _, key := range bucket.keys() {
		if strings.HasPrefix(key, built) {
			t.Errorf("%q survived a discarded generation", key)
		}
	}
	if strings.Contains(strings.Join(bucket.deleted, " "), superseded) {
		t.Error("the generation that is still serving was deleted")
	}
	if len(db.failed) != 0 {
		t.Error("a lost claim was recorded as a failure against a row that is no longer this job's")
	}
}

// A tile that cannot be written abandons the whole generation: the prefix goes
// and the row is marked failed, with the attempt counted against it.
func TestAFailedUploadThrowsAwayTheGeneration(t *testing.T) {
	w, db, bucket, log, asset := newWorker(t)
	bucket.putErrOn = "/z1/"

	w.tile(context.Background(), asset)

	built := storage.MapGenerationPrefix(testOwner, testAsset, testLease)
	if len(bucket.deleted) != 1 || bucket.deleted[0] != built {
		t.Errorf("deleted %v, want the half-built generation", bucket.deleted)
	}
	if len(bucket.keys()) != 0 {
		t.Errorf("%v survived the failure", bucket.keys())
	}
	if len(db.completed) != 0 {
		t.Error("a failed build published a row")
	}
	if len(db.failed) != 1 {
		t.Fatalf("recorded %d failures, want one", len(db.failed))
	}
	if db.failed[0].TileLease == nil || *db.failed[0].TileLease != testLease {
		t.Error("the failure does not name the lease, so it could land on a claim that replaced it")
	}
	if log.indexOf(t, "delete "+built) > log.indexOf(t, "fail") {
		t.Errorf("the row was failed before its objects were gone: %v", log.all())
	}
}

// THIS IS THE LEDGER RULE. A generation that could not be deleted is still
// named by the lease on its row, and that lease is the only record it exists.
// Marking the row failed would clear the lease and orphan the objects, so the
// row is left working instead and reclaim comes back for it.
func TestAGenerationThatCannotBeDeletedKeepsItsLease(t *testing.T) {
	w, db, bucket, _, asset := newWorker(t)
	bucket.putErrOn = "/z1/"
	bucket.deleteErrOn = testLease.String()

	w.tile(context.Background(), asset)

	if len(db.failed) != 0 {
		t.Error("the row was failed while its half-built generation was still in the bucket")
	}
	if len(db.completed) != 0 {
		t.Error("a failed build published a row")
	}
}

// Shutdown is not a failure. The row keeps its claim, nothing is deleted and no
// attempt is counted, because reclaim will pick the whole thing up when the
// process comes back.
func TestACancelledBuildIsNotAFailure(t *testing.T) {
	w, db, bucket, _, asset := newWorker(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w.tile(ctx, asset)

	if len(db.failed) != 0 {
		t.Error("a shutdown counted an attempt against the row")
	}
	if len(bucket.deleted) != 0 {
		t.Errorf("a shutdown deleted %v", bucket.deleted)
	}
}

// An original that cannot be read never reaches the tiler, and is still a
// failed attempt: nothing was written, so the empty prefix is deleted and the
// row is counted against.
func TestAnUnreadableOriginalFailsTheJob(t *testing.T) {
	w, db, bucket, _, asset := newWorker(t)
	bucket.getErr = errors.New("the bucket is down")

	w.tile(context.Background(), asset)

	if len(db.failed) != 1 {
		t.Errorf("recorded %d failures, want one", len(db.failed))
	}
	if len(bucket.keys()) != 0 {
		t.Errorf("%v was written for a job that could not read its source", bucket.keys())
	}
}

// The pool is what bounds how far the tiler may run ahead of the encoders, and
// so what bounds how many decoded tiles are alive at once. Without it a whole
// pyramid of them would pile up in front of R2.
func TestNoMoreThanThePoolIsInFlightAtOnce(t *testing.T) {
	w, _, bucket, _, asset := newWorker(t)

	w.tile(context.Background(), asset)

	if bucket.peak > encoders {
		t.Errorf("%d uploads were in flight at once, want at most %d", bucket.peak, encoders)
	}
	if bucket.peak < 2 {
		t.Errorf("peak concurrency was %d; the pool is not overlapping anything", bucket.peak)
	}
}

// Reclaim deletes a stranded job's half-built generation before putting the row
// back, and the requeue names the lease so it cannot land on a claim that
// replaced it.
func TestReclaimDeletesTheStrandedGenerationFirst(t *testing.T) {
	log := &journal{}
	bucket := &fakeBucket{log: log}
	db := &fakeRows{log: log, stranded: []queries.ListStrandedTilingJobsRow{
		{ID: testAsset, OwnerID: testOwner, TileLease: &testLease},
	}}
	w := &worker{db: db, bucket: bucket}

	w.reclaim(context.Background())

	stranded := storage.MapGenerationPrefix(testOwner, testAsset, testLease)
	if len(bucket.deleted) != 1 || bucket.deleted[0] != stranded {
		t.Errorf("deleted %v, want the stranded generation", bucket.deleted)
	}
	if len(db.requeued) != 1 {
		t.Fatalf("requeued %d rows, want one", len(db.requeued))
	}
	if db.requeued[0].TileLease == nil || *db.requeued[0].TileLease != testLease {
		t.Error("the requeue does not name the lease it is reclaiming")
	}
	if log.indexOf(t, "delete "+stranded) > log.indexOf(t, "requeue") {
		t.Errorf("the row was requeued before its objects were gone: %v", log.all())
	}
}

// The same rule as the failure path, from the other side: a stranded prefix
// that will not delete keeps its row working, because the lease is what will
// find it next time.
func TestReclaimLeavesARowWorkingWhenItsPrefixWillNotDelete(t *testing.T) {
	log := &journal{}
	bucket := &fakeBucket{log: log, deleteErrOn: testLease.String()}
	db := &fakeRows{log: log, stranded: []queries.ListStrandedTilingJobsRow{
		{ID: testAsset, OwnerID: testOwner, TileLease: &testLease},
	}}
	w := &worker{db: db, bucket: bucket}

	w.reclaim(context.Background())

	if len(db.requeued) != 0 {
		t.Error("a row was requeued while its generation was still in the bucket")
	}
}

// A failed row owns nothing in the bucket, so its retry is one statement and no
// R2 work. The cap and the cutoff are what make it a retry rather than a loop.
func TestReclaimRetriesFailedRowsUnderTheCap(t *testing.T) {
	log := &journal{}
	bucket := &fakeBucket{log: log}
	db := &fakeRows{log: log}
	w := &worker{db: db, bucket: bucket}

	w.reclaim(context.Background())
	ran := time.Now()

	if len(db.retried) != 1 {
		t.Fatalf("ran %d retry statements, want one", len(db.retried))
	}
	if db.retried[0].TileAttempts != MaxAttempts {
		t.Errorf("retried rows under %d attempts, want %d", db.retried[0].TileAttempts, MaxAttempts)
	}
	cutoff := db.retried[0].TileLeasedAt
	if !cutoff.Valid {
		t.Fatal("the retry has no cutoff, so every failure would be retried at once")
	}
	if waited := ran.Sub(cutoff.Time); waited < leaseWindow {
		t.Errorf("the retry cutoff is %v old, want at least a lease window of %v", waited, leaseWindow)
	}
	if len(bucket.deleted) != 0 {
		t.Errorf("a failed row's retry touched the bucket: %v", bucket.deleted)
	}
}

// A pass claims until the queue is empty, and reads the row back through the
// lease it just stamped -- there is no RETURNING to read it from.
func TestAPassClaimsUntilTheQueueIsEmpty(t *testing.T) {
	log := &journal{}
	bucket := &fakeBucket{log: log, original: sourcePNG(t)}
	db := &fakeRows{
		log:           log,
		claimed:       1,
		publishedRows: 1,
		asset: queries.Asset{
			ID:       testAsset,
			OwnerID:  testOwner,
			FilePath: storage.MapOriginalKey(testOwner, testAsset),
			Type:     queries.AssetsTypeMap,
			TileSize: sql.NullInt16{Int16: sourceTile, Valid: true},
		},
	}
	w := &worker{db: db, bucket: bucket, reclaimedAt: time.Now()}

	w.pass(context.Background())

	if len(db.completed) != 1 {
		t.Fatalf("tiled %d maps, want one", len(db.completed))
	}
	// The lease is minted by the claim, so the generation the tiles landed
	// under is whatever it stamped -- and the publish has to name that.
	published := db.completed[0]
	if published.TileGen == nil {
		t.Fatal("published no generation")
	}
	if _, ok := bucket.stored[storage.MapTileKey(testOwner, testAsset, *published.TileGen, 0, 0, 0)]; !ok {
		t.Error("the published generation is not the one the tiles were written under")
	}
}

// THE RAMP'S THREE FIXED POINTS, which are the whole of what was asked for:
// the level that is zoomed all the way in keeps the most, the level that is the
// whole map on one screen keeps the least, and the middle of the pyramid sits
// between them. A ramp that ran the other way would look like nothing at all
// until somebody zoomed in on a coastline.
func TestTileQualityFallsWithTheLevel(t *testing.T) {
	const maxZoom = 5

	if got := tileQuality(0, maxZoom); got != 90 {
		t.Errorf("level 0 encoded at %d, want 90", got)
	}
	if got := tileQuality(maxZoom, maxZoom); got != 70 {
		t.Errorf("the top level encoded at %d, want 70", got)
	}

	// Halfway up a six-level pyramid is level 2 or 3, and both are meant to be
	// about 80 rather than exactly it -- the ramp is a straight line through
	// integers, not a table of three values.
	for _, z := range []int{2, 3} {
		if got := tileQuality(z, maxZoom); got < 77 || got > 83 {
			t.Errorf("level %d of %d encoded at %d, want about 80", z, maxZoom, got)
		}
	}

	// And it never climbs on the way up, whatever the rounding does.
	for z := 1; z <= maxZoom; z++ {
		if tileQuality(z, maxZoom) > tileQuality(z-1, maxZoom) {
			t.Fatalf("level %d encoded higher than level %d", z, z-1)
		}
	}
}

// A map small enough to fit in one tile has no ramp to run down: its only level
// IS the native one, and dividing by a zero max zoom would panic on the way to
// deciding that.
func TestTileQualityOfAOneLevelPyramid(t *testing.T) {
	if got := tileQuality(0, 0); got != 90 {
		t.Errorf("the only level of a one-level pyramid encoded at %d, want 90", got)
	}
}
