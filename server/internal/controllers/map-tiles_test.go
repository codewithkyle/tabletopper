package controllers

import (
	"bytes"
	"database/sql"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/tiler"

	"github.com/oklog/ulid/v2"
)

var (
	testTileGen   = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS3")
	testStaleGen  = ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS4")
	testTileRoute = "/assets/maps/" + testAssetID.String() + "/tiles/" + testTileGen.String() + "/0/0_0.webp"
)

// servingPyramid is the plan's worked example -- a 12000x9000 map at 512 --
// which is six levels deep and has a remainder on both axes at every one of
// them. Its grid is 24x18, 12x9, 6x5, 3x3, 2x2, 1x1.
func servingPyramid() queries.GetMapPyramidRow {
	gen := testTileGen
	return queries.GetMapPyramidRow{
		OwnerID:  testOwnerID,
		Width:    sql.NullInt32{Int32: 12000, Valid: true},
		Height:   sql.NullInt32{Int32: 9000, Valid: true},
		TileSize: sql.NullInt16{Int16: 512, Valid: true},
		MaxZoom:  sql.NullInt16{Int16: 5, Valid: true},
		TileGen:  &gen,
	}
}

// tileRequest builds the request the route would have built, with the path
// values the mux would have filled in.
func tileRequest(id string, gen string, z string, name string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, testTileRoute, nil)
	r.SetPathValue("id", id)
	r.SetPathValue("gen", gen)
	r.SetPathValue("z", z)
	r.SetPathValue("tile", name)
	return r
}

// A TILE HAS ONE URL AND NOT FOUR. These paths are cached for a year and never
// revalidated, so every spelling that Atoi would accept as the same number is a
// second copy of the same bytes in every browser and every proxy between here
// and the table.
func TestATileURLIsReadInExactlyOneForm(t *testing.T) {
	for name, c := range map[string]struct {
		gen, z, tile string
		want         bool
	}{
		"the canonical form":              {testTileGen.String(), "3", "2_1.webp", true},
		"zero everywhere":                 {testTileGen.String(), "0", "0_0.webp", true},
		"a multi-digit index":             {testTileGen.String(), "0", "23_17.webp", true},
		"a padded level":                  {testTileGen.String(), "03", "2_1.webp", false},
		"a padded index":                  {testTileGen.String(), "3", "02_1.webp", false},
		"a signed index":                  {testTileGen.String(), "3", "+2_1.webp", false},
		"a negative index":                {testTileGen.String(), "3", "-2_1.webp", false},
		"a spaced index":                  {testTileGen.String(), "3", "2_ 1.webp", false},
		"a fractional index":              {testTileGen.String(), "3", "2_1.5.webp", false},
		"no extension":                    {testTileGen.String(), "3", "2_1", false},
		"the wrong extension":             {testTileGen.String(), "3", "2_1.png", false},
		"no separator":                    {testTileGen.String(), "3", "21.webp", false},
		"an empty index":                  {testTileGen.String(), "3", "2_.webp", false},
		"nothing but a separator":         {testTileGen.String(), "3", "_.webp", false},
		"a generation that is not a ULID": {"not-a-ulid", "3", "2_1.webp", false},
		"an empty generation":             {"", "3", "2_1.webp", false},
	} {
		t.Run(name, func(t *testing.T) {
			tile, ok := parseTileCoords(c.gen, c.z, c.tile)
			if ok != c.want {
				t.Fatalf("parseTileCoords(%q, %q, %q) ok = %v, want %v", c.gen, c.z, c.tile, ok, c.want)
			}
			if !ok {
				return
			}
			if tile.gen != testTileGen {
				t.Errorf("gen = %v, want %v", tile.gen, testTileGen)
			}
		})
	}
}

// The whole of the validation, against the pyramid the row describes. Getting
// this wrong in one direction hands out 404s for tiles that are sitting in the
// bucket; in the other it sends R2 a key nothing ever wrote.
func TestATileMustBeInsideThePyramidTheRowDescribes(t *testing.T) {
	stale := testStaleGen

	for name, c := range map[string]struct {
		row     queries.GetMapPyramidRow
		z, x, y int
		want    bool
	}{
		"the first tile of the native level":  {servingPyramid(), 0, 0, 0, true},
		"the last tile of the native level":   {servingPyramid(), 0, 23, 17, true},
		"one column past the native level":    {servingPyramid(), 0, 24, 17, false},
		"one row past the native level":       {servingPyramid(), 0, 23, 18, false},
		"the last tile of a level that ceils": {servingPyramid(), 2, 5, 4, true},
		"one column past that level":          {servingPyramid(), 2, 6, 4, false},
		"one row past that level":             {servingPyramid(), 2, 5, 5, false},
		"the single tile at the top":          {servingPyramid(), 5, 0, 0, true},
		"a second tile at the top":            {servingPyramid(), 5, 1, 0, false},
		"a level above the top":               {servingPyramid(), 6, 0, 0, false},
		"a level far above the top":           {servingPyramid(), 4000, 0, 0, false},
		"a negative level":                    {servingPyramid(), -1, 0, 0, false},
		"a negative column":                   {servingPyramid(), 0, -1, 0, false},
		"a negative row":                      {servingPyramid(), 0, 0, -1, false},

		// A generation that is not the one serving is a pyramid that has been
		// deleted, or one that is still being built. Neither is in the bucket
		// under a name this row will admit to.
		"a superseded generation": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.TileGen = &stale; return r }(),
			0, 0, 0, false,
		},
		// Nothing has ever completed, so there is nothing to serve at all --
		// which is the state every map is in between its upload and its first
		// build finishing.
		"a map that has never been tiled": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.TileGen = nil; return r }(),
			0, 0, 0, false,
		},
		// The four pyramid columns are written in the same statement that sets
		// tile_gen, so a row missing one of them is a row that cannot be
		// trusted about any of them.
		"a generation with no width": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.Width = sql.NullInt32{}; return r }(),
			0, 0, 0, false,
		},
		"a generation with no height": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.Height = sql.NullInt32{}; return r }(),
			0, 0, 0, false,
		},
		"a generation with no tile size": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.TileSize = sql.NullInt16{}; return r }(),
			0, 0, 0, false,
		},
		"a generation with no max zoom": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.MaxZoom = sql.NullInt16{}; return r }(),
			0, 0, 0, false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tile := tileCoords{gen: testTileGen, z: c.z, x: c.x, y: c.y}
			if got := tileExists(c.row, tile); got != c.want {
				t.Errorf("tileExists(z=%d, x=%d, y=%d) = %v, want %v", c.z, c.x, c.y, got, c.want)
			}
		})
	}
}

// The read is by id alone and takes the pyramid columns only. It is not scoped
// to the owner, because everyone at the table fetches the DM's map, and it does
// not look at tile_state, because a map being re-tiled goes on serving the
// generation it already has.
func TestTheTileRouteReadsThePyramidAndNotTheJob(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}
	rec := httptest.NewRecorder()

	app.GetMapTile(rec, tileRequest(testAssetID.String(), testTileGen.String(), "0", "0_0.webp"))

	if len(db.reads) != 1 {
		t.Fatalf("ran %d reads, want 1", len(db.reads))
	}
	read := db.reads[0]
	for _, want := range []string{"tile_gen", "owner_id", "max_zoom", "WHERE id = ?"} {
		if !strings.Contains(read.query, want) {
			t.Errorf("the read does not contain %q: %q", want, read.query)
		}
	}
	if strings.Contains(read.query, "tile_state") {
		t.Errorf("the read consults the job's state, so a re-tile would stop serving: %q", read.query)
	}
	if strings.Contains(read.query, "owner_id = ?") {
		t.Errorf("the read is scoped to an owner, so only the DM could load the map: %q", read.query)
	}
	if len(read.args) != 1 || read.args[0] != testAssetID {
		t.Errorf("the read ran with %v, want just the asset", read.args)
	}
	if len(db.calls) != 0 {
		t.Errorf("the route wrote %d statements, want 0", len(db.calls))
	}
}

// EVERY FAILURE IS THE STATUS AND NOTHING ELSE. The caller is a renderer's
// fetch or an <img>, and http.NotFound would put "404 page not found" where the
// tile was meant to be. The immutable header must not come with it either: a
// year-long cache entry for a tile that does not exist would outlive the build
// that would have created it.
func TestAMissingTileIsAnEmptyBodyAndNoCacheHeader(t *testing.T) {
	db := &recordingDB{}
	app := &App{Queries: queries.New(db)}
	rec := httptest.NewRecorder()

	app.GetMapTile(rec, tileRequest(testAssetID.String(), testTileGen.String(), "0", "0_0.webp"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("body = %q, want nothing at all", body)
	}
	if cache := rec.Header().Get("Cache-Control"); cache != "" {
		t.Errorf("Cache-Control = %q on a tile that does not exist, want nothing", cache)
	}
}

// A URL the route cannot read is answered before anything is queried. Every one
// of these is a shape the mux happily matched -- the last segment is one
// wildcard, so the mux checks nothing about what is in it.
func TestAMalformedTileURLNeverReachesTheDatabase(t *testing.T) {
	for name, c := range map[string]struct{ id, gen, z, tile string }{
		"an asset that is not a ULID":     {"not-a-ulid", testTileGen.String(), "0", "0_0.webp"},
		"a generation that is not a ULID": {testAssetID.String(), "not-a-ulid", "0", "0_0.webp"},
		"a level that is not a number":    {testAssetID.String(), testTileGen.String(), "top", "0_0.webp"},
		"indexes that are not numbers":    {testAssetID.String(), testTileGen.String(), "0", "a_b.webp"},
		"a segment that is not a tile":    {testAssetID.String(), testTileGen.String(), "0", "preview"},
		"a path traversal in the segment": {testAssetID.String(), testTileGen.String(), "0", "../original"},
	} {
		t.Run(name, func(t *testing.T) {
			db := &recordingDB{}
			app := &App{Queries: queries.New(db)}
			rec := httptest.NewRecorder()

			app.GetMapTile(rec, tileRequest(c.id, c.gen, c.z, c.tile))

			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
			if len(db.reads) != 0 {
				t.Errorf("ran %d reads, want 0", len(db.reads))
			}
		})
	}
}

// THE ROUTE AND THE TILER HAVE TO AGREE EXACTLY, and nothing but this checks
// that they do. The tiler decides which tiles exist by walking the pyramid; the
// route decides which tiles exist by arithmetic on four columns. They are the
// same arithmetic today because both go through internal/tiler, and if they
// ever stopped being, the symptom is a 404 for a tile that is sitting in the
// bucket -- a hole in a map, with no error logged anywhere.
//
// The row is filled from the tiler's Result exactly as CompleteMapTiling fills
// it, so this walks the same values the worker would have written.
func TestEveryTileTheTilerBuildsIsOneTheRouteWillServe(t *testing.T) {
	const tileSize = 128

	// 1000x700 leaves a remainder on both axes at every level, which is what
	// makes the ceil in the pyramid arithmetic load-bearing: 8x6, 4x3, 2x2,
	// 1x1.
	var source bytes.Buffer
	if err := png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 1000, 700))); err != nil {
		t.Fatalf("encoding the source: %v", err)
	}

	built := map[tileCoords]bool{}
	result, err := tiler.Build(&source, tileSize, func(tile tiler.Tile) error {
		built[tileCoords{gen: testTileGen, z: tile.Z, x: tile.X, y: tile.Y}] = true
		return nil
	})
	if err != nil {
		t.Fatalf("building the pyramid: %v", err)
	}

	gen := testTileGen
	row := queries.GetMapPyramidRow{
		OwnerID:  testOwnerID,
		Width:    sql.NullInt32{Int32: int32(result.Width), Valid: true},
		Height:   sql.NullInt32{Int32: int32(result.Height), Valid: true},
		TileSize: sql.NullInt16{Int16: int16(result.TileSize), Valid: true},
		MaxZoom:  sql.NullInt16{Int16: int16(result.MaxZoom), Valid: true},
		TileGen:  &gen,
	}

	if len(built) == 0 {
		t.Fatal("the tiler emitted nothing")
	}
	for tile := range built {
		if !tileExists(row, tile) {
			t.Errorf("the tiler built z%d %d_%d and the route will not serve it", tile.z, tile.x, tile.y)
		}
	}

	// And nothing beyond them. The walk goes one level past the top and one
	// tile past each edge of every level, which is every way a renderer asks
	// for a tile that was never built.
	for z := 0; z <= result.MaxZoom+1; z++ {
		for x := 0; x <= tiler.LevelTiles(result.Width, tileSize, z); x++ {
			for y := 0; y <= tiler.LevelTiles(result.Height, tileSize, z); y++ {
				tile := tileCoords{gen: gen, z: z, x: x, y: y}
				if !built[tile] && tileExists(row, tile) {
					t.Errorf("the route serves z%d %d_%d and the tiler never built it", z, x, y)
				}
			}
		}
	}
}
