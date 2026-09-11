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
func tileRequest(id string, gen string, z string, name string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, testTileRoute, nil)
	r.SetPathValue("id", id)
	r.SetPathValue("gen", gen)
	r.SetPathValue("z", z)
	r.SetPathValue("tile", name)
	return r
}
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
		"a superseded generation": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.TileGen = &stale; return r }(),
			0, 0, 0, false,
		},
		"a map that has never been tiled": {
			func() queries.GetMapPyramidRow { r := servingPyramid(); r.TileGen = nil; return r }(),
			0, 0, 0, false,
		},
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
func TestEveryTileTheTilerBuildsIsOneTheRouteWillServe(t *testing.T) {
	const tileSize = 128
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
