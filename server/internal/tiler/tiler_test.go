package tiler

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// coordinateImage paints every pixel with its own position, so a tile cut from
// the wrong offset shows up as the wrong pixel rather than as a plausible one.
// It is opaque, which is what makes a premultiplied RGBA tile comparable to it
// channel for channel.
func coordinateImage(width int, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encoding the test source: %v", err)
	}
	return out.Bytes()
}

// buildPNG runs a whole pyramid and collects it, in the order it was emitted.
func buildPNG(t *testing.T, img image.Image, tileSize int) (Result, []Tile) {
	t.Helper()
	var tiles []Tile
	result, err := Build(bytes.NewReader(encodePNG(t, img)), tileSize, func(tile Tile) error {
		tiles = append(tiles, tile)
		return nil
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return result, tiles
}

// A source that already fits in a tile is a pyramid of one level and one tile,
// and the tile is the image rather than a tile-sized canvas with the image in
// a corner.
func TestASinglePixelIsOneTile(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(1, 1), 8)

	if result.Width != 1 || result.Height != 1 || result.MaxZoom != 0 {
		t.Errorf("Result = %+v, want 1x1 at max zoom 0", result)
	}
	if len(tiles) != 1 {
		t.Fatalf("built %d tiles, want 1", len(tiles))
	}
	if got := tiles[0].Image.Bounds(); got.Dx() != 1 || got.Dy() != 1 {
		t.Errorf("the tile is %v, want 1x1", got)
	}
}

// A source exactly one tile across has one column and no remainder, which is
// the case an off-by-one in the tile count turns into a second, empty column.
func TestASourceExactlyOneTileWide(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(8, 20), 8)

	if result.MaxZoom != 2 {
		t.Errorf("MaxZoom = %d, want 2", result.MaxZoom)
	}
	var atZero []Tile
	for _, tile := range tiles {
		if tile.Z == 0 {
			atZero = append(atZero, tile)
		}
	}
	if len(atZero) != 3 {
		t.Fatalf("level 0 is %d tiles, want 3 -- one column of three", len(atZero))
	}
	for _, tile := range atZero {
		if tile.X != 0 {
			t.Errorf("level 0 tile at x=%d, want a single column", tile.X)
		}
	}
	if got := atZero[2].Image.Bounds().Dy(); got != 4 {
		t.Errorf("the bottom tile is %d tall, want 4 -- the remainder of 20", got)
	}
}

// One pixel past a tile boundary is a whole extra column of tiles, each of
// them one pixel wide. It is the ugliest case the arithmetic has, and the one
// that a tiler which padded its edges would hide.
func TestOnePixelOverATileBoundary(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(9, 8), 8)

	if result.MaxZoom != 1 {
		t.Errorf("MaxZoom = %d, want 1", result.MaxZoom)
	}

	var edge *Tile
	for i, tile := range tiles {
		if tile.Z == 0 && tile.X == 1 {
			edge = &tiles[i]
		}
	}
	if edge == nil {
		t.Fatal("no tile at level 0 x=1; the ninth column of pixels is in no tile")
	}
	if got := edge.Image.Bounds(); got.Dx() != 1 || got.Dy() != 8 {
		t.Errorf("the edge tile is %v, want 1x8", got)
	}
}

// The axes bottom out at different levels on anything long and thin, and the
// pyramid does not stop until both have. The top level here is 8x1.
func TestANonSquareSourceRunsToTheLongerAxis(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(64, 4), 8)

	if result.MaxZoom != 3 {
		t.Fatalf("MaxZoom = %d, want 3", result.MaxZoom)
	}
	top := tiles[len(tiles)-1]
	if top.Z != 3 || top.X != 0 || top.Y != 0 {
		t.Errorf("the last tile is z%d %d,%d, want z3 0,0", top.Z, top.X, top.Y)
	}
	if got := top.Image.Bounds(); got.Dx() != 8 || got.Dy() != 1 {
		t.Errorf("the top tile is %v, want 8x1", got)
	}
}

// Every level is covered exactly once, with no gap and no overlap, and the
// widths of a row's tiles add up to the level's width. A missing edge tile is
// a strip of the map that never loads; an extra one is a request that 404s.
func TestEveryLevelIsCoveredExactlyOnce(t *testing.T) {
	const width, height, tileSize = 37, 21, 8
	result, tiles := buildPNG(t, coordinateImage(width, height), tileSize)

	seen := map[[3]int]bool{}
	for _, tile := range tiles {
		key := [3]int{tile.Z, tile.X, tile.Y}
		if seen[key] {
			t.Errorf("tile z%d %d,%d was emitted twice", tile.Z, tile.X, tile.Y)
		}
		seen[key] = true
	}

	for z := 0; z <= result.MaxZoom; z++ {
		across := LevelTiles(width, tileSize, z)
		down := LevelTiles(height, tileSize, z)

		var spannedX, spannedY int
		for y := 0; y < down; y++ {
			for x := 0; x < across; x++ {
				if !seen[[3]int{z, x, y}] {
					t.Fatalf("tile z%d %d,%d is missing", z, x, y)
				}
			}
		}
		for x := 0; x < across; x++ {
			spannedX += LevelTileSize(width, tileSize, z, x)
		}
		for y := 0; y < down; y++ {
			spannedY += LevelTileSize(height, tileSize, z, y)
		}
		if spannedX != LevelPixels(width, z) || spannedY != LevelPixels(height, z) {
			t.Errorf("level %d tiles span %dx%d, want %dx%d",
				z, spannedX, spannedY, LevelPixels(width, z), LevelPixels(height, z))
		}
	}

	if len(tiles) != len(seen) {
		t.Errorf("emitted %d tiles for %d positions", len(tiles), len(seen))
	}
}

// Level 0 is a crop of the source, so every pixel of every tile has to be the
// pixel the source had at that position. An origin that was off by a tile, or
// by the source's bounds, would still produce a plausible-looking map -- one
// that is shifted.
func TestLevelZeroCarriesTheSourcePixels(t *testing.T) {
	const width, height, tileSize = 21, 13, 8
	source := coordinateImage(width, height)
	_, tiles := buildPNG(t, source, tileSize)

	for _, tile := range tiles {
		if tile.Z != 0 {
			continue
		}
		bounds := tile.Image.Bounds()
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				got := tile.Image.RGBAAt(x, y)
				want := source.NRGBAAt(tile.X*tileSize+x, tile.Y*tileSize+y)
				if got.R != want.R || got.G != want.G || got.B != want.B || got.A != want.A {
					t.Fatalf("tile %d,%d pixel %d,%d = %v, want %v", tile.X, tile.Y, x, y, got, want)
				}
			}
		}
	}
}

// The halving is a 2x2 box average: each pixel of a level is the mean of the
// four it covers below. A filter that sampled instead would alias hard grid
// lines into moire, and one that rang would halo them.
func TestHalvingIsTheFourPixelMean(t *testing.T) {
	// Values chosen so every 2x2 block's mean is a whole number and no two
	// blocks share one, which a nearest-neighbour filter could not fake.
	source := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(16 * (y*4 + x)), A: 255})
		}
	}

	// A tile size of 2 is what puts a level 1 above a 4x4 source at all.
	_, tiles := buildPNG(t, source, 2)

	var level1 *Tile
	for i, tile := range tiles {
		if tile.Z == 1 {
			level1 = &tiles[i]
		}
	}
	if level1 == nil {
		t.Fatal("no level 1 was emitted")
	}

	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			sum := 0
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					sum += int(source.NRGBAAt(x*2+dx, y*2+dy).R)
				}
			}
			if got := int(level1.Image.RGBAAt(x, y).R); got != sum/4 {
				t.Errorf("level 1 pixel %d,%d = %d, want the mean %d", x, y, got, sum/4)
			}
		}
	}
}

// Level 0 first, and every level complete before the next begins. The caller
// writes tiles as they arrive, so the order is what decides whether a pyramid
// that is interrupted has a usable bottom or a scattering of levels.
func TestTilesArriveLevelZeroFirst(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(37, 21), 8)

	previous := 0
	for _, tile := range tiles {
		if tile.Z != previous && tile.Z != previous+1 {
			t.Fatalf("level %d arrived after level %d", tile.Z, previous)
		}
		previous = tile.Z
	}
	if tiles[0].Z != 0 {
		t.Errorf("the first tile is at level %d, want 0", tiles[0].Z)
	}
	if previous != result.MaxZoom {
		t.Errorf("the last tile is at level %d, want %d", previous, result.MaxZoom)
	}
}

// An emit that fails stops the build where it is and comes back unwrapped, so
// the caller can tell its own failure from a decode it cannot do anything
// about. This is the whole cancellation story: there is no other way in.
func TestAFailingEmitAbandonsTheBuild(t *testing.T) {
	stop := errors.New("the pool gave up")

	emitted := 0
	_, err := Build(bytes.NewReader(encodePNG(t, coordinateImage(64, 64))), 8, func(Tile) error {
		emitted++
		if emitted == 3 {
			return stop
		}
		return nil
	})

	if !errors.Is(err, stop) {
		t.Errorf("Build = %v, want the emit error back", err)
	}
	if emitted != 3 {
		t.Errorf("emit was called %d times, want it to stop at 3", emitted)
	}
}

// jpegWithOrientation encodes img and splices an EXIF block carrying a single
// Orientation tag in behind the SOI marker, which is where a camera would have
// put it.
func jpegWithOrientation(t *testing.T, img image.Image, orientation uint16) []byte {
	t.Helper()

	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, nil); err != nil {
		t.Fatalf("encoding the test source: %v", err)
	}

	exif := []byte{
		'E', 'x', 'i', 'f', 0x00, 0x00,
		'M', 'M', 0x00, 0x2a, // big endian, and the TIFF magic
		0x00, 0x00, 0x00, 0x08, // the first directory follows immediately
		0x00, 0x01, // holding one tag
		0x01, 0x12, // Orientation
		0x00, 0x03, // as a SHORT
		0x00, 0x00, 0x00, 0x01, // one of them
		byte(orientation >> 8), byte(orientation), 0x00, 0x00, // left-aligned in its four bytes
		0x00, 0x00, 0x00, 0x00, // and no directory after this one
	}

	out := make([]byte, 0, raw.Len()+len(exif)+4)
	out = append(out, raw.Bytes()[:2]...) // the SOI marker
	out = append(out, 0xff, 0xe1, byte((len(exif)+2)>>8), byte(len(exif)+2))
	out = append(out, exif...)
	return append(out, raw.Bytes()[2:]...)
}

// A PHONE PHOTO OF A HAND-DRAWN MAP IS THE CASE THIS EXISTS FOR. The file's
// header says 40x20 either way; only the orientation tag says which way up it
// is. The pyramid is laid out over the pixels as seen, so the dimensions the
// asset row records have to be the rotated ones -- a row that disagreed with
// its own pyramid would have the renderer asking for tiles that are not there.
func TestAnOrientationTagRotatesThePyramid(t *testing.T) {
	source := coordinateImage(40, 20)

	rotated, err := Build(bytes.NewReader(jpegWithOrientation(t, source, 6)), 8, func(Tile) error { return nil })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rotated.Width != 20 || rotated.Height != 40 {
		t.Errorf("a rotated source built a %dx%d pyramid, want 20x40", rotated.Width, rotated.Height)
	}

	// The control: the same splice carrying the tag that means "as stored".
	upright, err := Build(bytes.NewReader(jpegWithOrientation(t, source, 1)), 8, func(Tile) error { return nil })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if upright.Width != 40 || upright.Height != 20 {
		t.Errorf("an upright source built a %dx%d pyramid, want 40x20", upright.Width, upright.Height)
	}
}

func TestBuildRefusesWhatItCannotBuild(t *testing.T) {
	source := encodePNG(t, coordinateImage(8, 8))

	if _, err := Build(bytes.NewReader(source), 0, func(Tile) error { return nil }); err == nil {
		t.Error("Build with a tile size of 0 = nil, want an error")
	}
	if _, err := Build(bytes.NewReader(source), 8, nil); err == nil {
		t.Error("Build with no emit = nil, want an error")
	}
	if _, err := Build(bytes.NewReader([]byte("not an image")), 8, func(Tile) error { return nil }); err == nil {
		t.Error("Build of a non-image = nil, want an error")
	}
}

// EVERY TILE CARRIES THE PYRAMID'S TOP LEVEL, which is what lets a consumer
// decide something about a tile while the build is still running. The encoder
// pool is handed tiles through a channel and never sees the Result, so a tile
// that only knew its own Z could not tell the bottom of the pyramid from the
// top of it.
func TestEveryTileKnowsHowTallThePyramidIs(t *testing.T) {
	result, tiles := buildPNG(t, coordinateImage(40, 24), 8)

	if result.MaxZoom < 2 {
		t.Fatalf("the fixture is only %d levels tall; it cannot show this", result.MaxZoom+1)
	}
	for _, tile := range tiles {
		if tile.MaxZoom != result.MaxZoom {
			t.Fatalf("tile z%d %d,%d says the pyramid is %d tall, want %d",
				tile.Z, tile.X, tile.Y, tile.MaxZoom, result.MaxZoom)
		}
	}
}
