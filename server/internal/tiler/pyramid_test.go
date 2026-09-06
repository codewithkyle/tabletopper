package tiler

import (
	"fmt"
	"testing"
)

// The pyramid a 12000x9000 map at 512 comes to, level by level. It is the
// worked example every other decision here was sized against -- 584 objects
// and a max_zoom of 5 -- so it is pinned rather than recomputed by whoever
// next wonders whether a change moved it.
func TestTwelveThousandByNineThousandAtFiveTwelve(t *testing.T) {
	const width, height, tileSize = 12000, 9000, 512

	levels := []struct {
		pixelsX, pixelsY int
		tilesX, tilesY   int
	}{
		{12000, 9000, 24, 18},
		{6000, 4500, 12, 9},
		{3000, 2250, 6, 5},
		{1500, 1125, 3, 3},
		{750, 563, 2, 2},
		{375, 282, 1, 1},
	}

	if got := MaxZoom(width, height, tileSize); got != len(levels)-1 {
		t.Fatalf("MaxZoom = %d, want %d", got, len(levels)-1)
	}

	total := 0
	for z, want := range levels {
		if got := LevelPixels(width, z); got != want.pixelsX {
			t.Errorf("LevelPixels(width, %d) = %d, want %d", z, got, want.pixelsX)
		}
		if got := LevelPixels(height, z); got != want.pixelsY {
			t.Errorf("LevelPixels(height, %d) = %d, want %d", z, got, want.pixelsY)
		}
		if got := LevelTiles(width, tileSize, z); got != want.tilesX {
			t.Errorf("LevelTiles(width, %d) = %d, want %d", z, got, want.tilesX)
		}
		if got := LevelTiles(height, tileSize, z); got != want.tilesY {
			t.Errorf("LevelTiles(height, %d) = %d, want %d", z, got, want.tilesY)
		}
		total += want.tilesX * want.tilesY
	}

	if total != 584 {
		t.Errorf("pyramid is %d tiles, want 584", total)
	}
}

// THIS IS THE BUG THE CEIL EXISTS TO PREVENT. Halving 9000 five times with a
// bare right shift gives 281, and 281 rows at level 5 leave the last row of
// the map in no tile at all. Nothing reports it: the level renders, one row
// short.
func TestLevelPixelsCeilsRatherThanFloors(t *testing.T) {
	if got := LevelPixels(9000, 5); got != 282 {
		t.Errorf("LevelPixels(9000, 5) = %d, want 282", got)
	}
	if floored := 9000 >> 5; floored != 281 {
		t.Fatalf("the premise moved: 9000>>5 = %d", floored)
	}
}

// The tiler halves the level below it, one step at a time; the tile route and
// the renderer compute a level straight from the native size. The two have to
// agree at every level of every image, and they only do because ceiling twice
// by two is ceiling once by four.
func TestSuccessiveHalvingAgreesWithTheClosedForm(t *testing.T) {
	for _, size := range []int{1, 2, 3, 7, 9, 255, 512, 513, 999, 4500, 9000, 12000, 16383} {
		stepped := size
		for z := 1; z <= 20; z++ {
			stepped = (stepped + 1) / 2
			if got := LevelPixels(size, z); got != stepped {
				t.Errorf("LevelPixels(%d, %d) = %d, halving step by step gives %d", size, z, got, stepped)
			}
		}
	}
}

// max_zoom is where BOTH axes fit in one tile, not where either does. A 64x4
// strip at 8 has a height that fits from the start and a width that takes
// three halvings; stopping at the first axis to fit would leave a level that
// is still eight tiles wide as the top of the pyramid.
func TestMaxZoomWaitsForTheLongerAxis(t *testing.T) {
	if got := MaxZoom(64, 4, 8); got != 3 {
		t.Errorf("MaxZoom(64, 4, 8) = %d, want 3", got)
	}
	if got := MaxZoom(1, 1, 8); got != 0 {
		t.Errorf("MaxZoom(1, 1, 8) = %d, want 0", got)
	}
	if got := MaxZoom(8, 8, 8); got != 0 {
		t.Errorf("MaxZoom(8, 8, 8) = %d, want 0", got)
	}
	if got := MaxZoom(9, 8, 8); got != 1 {
		t.Errorf("MaxZoom(9, 8, 8) = %d, want 1", got)
	}
}

// The last tile of a row is the remainder, and the remainder can be one pixel.
// A tiler that padded instead would draw a seam down the right edge of every
// map whose width is not a multiple of the tile size, which is most of them.
func TestLevelTileSizeIsTheRemainderAtTheEdge(t *testing.T) {
	// 17 at a tile size of 8 is 8, 8, 1.
	for x, want := range []int{8, 8, 1} {
		if got := LevelTileSize(17, 8, 0, x); got != want {
			t.Errorf("LevelTileSize(17, 8, 0, %d) = %d, want %d", x, got, want)
		}
	}
	// 16 divides evenly, so there is no remainder and no third tile.
	for x, want := range []int{8, 8} {
		if got := LevelTileSize(16, 8, 0, x); got != want {
			t.Errorf("LevelTileSize(16, 8, 0, %d) = %d, want %d", x, got, want)
		}
	}
	if got := LevelTileSize(16, 8, 0, 2); got != 0 {
		t.Errorf("LevelTileSize past the last tile = %d, want 0", got)
	}
}

// The tile route hands these whatever a URL contained, and it validates by
// asking them. So every degenerate argument has to come back as a number that
// makes the request invalid rather than as a panic or a wrap-around: zero
// tiles means no x is in range, which is the 404 the route wants.
func TestDegenerateArgumentsAnswerWithNoTiles(t *testing.T) {
	cases := []struct {
		size, tileSize, z int
	}{
		{0, 512, 0},
		{-1, 512, 0},
		{1024, 0, 0},
		{1024, -8, 0},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("size=%d,tile=%d", c.size, c.tileSize), func(t *testing.T) {
			if got := LevelTiles(c.size, c.tileSize, c.z); got != 0 {
				t.Errorf("LevelTiles = %d, want 0", got)
			}
			if got := LevelTileSize(c.size, c.tileSize, c.z, 0); got != 0 {
				t.Errorf("LevelTileSize = %d, want 0", got)
			}
		})
	}

	// A z far past the top of any pyramid is a single pixel, not a shift
	// wider than the word.
	for _, z := range []int{40, 62, 63, 1000} {
		if got := LevelPixels(12000, z); got != 1 {
			t.Errorf("LevelPixels(12000, %d) = %d, want 1", z, got)
		}
	}
	if got := MaxZoom(12000, 9000, 0); got != 0 {
		t.Errorf("MaxZoom with no tile size = %d, want 0", got)
	}
}
