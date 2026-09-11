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
func TestHalvingIsTheFourPixelMean(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(16 * (y*4 + x)), A: 255})
		}
	}
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
func jpegWithOrientation(t *testing.T, img image.Image, orientation uint16) []byte {
	t.Helper()
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, nil); err != nil {
		t.Fatalf("encoding the test source: %v", err)
	}
	exif := []byte{
		'E', 'x', 'i', 'f', 0x00, 0x00,
		'M', 'M', 0x00, 0x2a, 
		0x00, 0x00, 0x00, 0x08, 
		0x00, 0x01, 
		0x01, 0x12, 
		0x00, 0x03, 
		0x00, 0x00, 0x00, 0x01, 
		byte(orientation >> 8), byte(orientation), 0x00, 0x00, 
		0x00, 0x00, 0x00, 0x00, 
	}
	out := make([]byte, 0, raw.Len()+len(exif)+4)
	out = append(out, raw.Bytes()[:2]...) 
	out = append(out, 0xff, 0xe1, byte((len(exif)+2)>>8), byte(len(exif)+2))
	out = append(out, exif...)
	return append(out, raw.Bytes()[2:]...)
}
func TestAnOrientationTagRotatesThePyramid(t *testing.T) {
	source := coordinateImage(40, 20)
	rotated, err := Build(bytes.NewReader(jpegWithOrientation(t, source, 6)), 8, func(Tile) error { return nil })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rotated.Width != 20 || rotated.Height != 40 {
		t.Errorf("a rotated source built a %dx%d pyramid, want 20x40", rotated.Width, rotated.Height)
	}
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
