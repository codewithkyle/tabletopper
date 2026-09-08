// Package tiler slices one image into a tile pyramid: level 0 at the image's
// native resolution, every level above it halved, up to the level that fits
// inside a single tile.
//
// It is image manipulation and nothing else -- no object store, no database,
// no encoding -- so what it produces can be checked against a synthetic image
// in a test rather than against a bucket. Encoding is deliberately the
// caller's: it is where the seconds go, so it is the caller that decides how
// many cores to spend on it.
package tiler

import (
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/disintegration/imaging"
	"golang.org/x/image/draw"
)

// Tile is one tile of one level, as decoded pixels.
//
// Image is *image.RGBA because that is the one type the WebP encoder takes
// without first copying it into one. It is tileSize square except along the
// right edge and the bottom, where it is the remainder.
//
// MAXZOOM RIDES ON EVERY TILE so that a consumer can tell how deep in the
// pyramid this one sits without waiting for Build to return -- and no consumer
// worth the callback can wait, because encoding runs alongside the build and
// the whole point of emit is that the tiles are handed over as they are cut.
// Z on its own says "level 3"; Z against MaxZoom says "the bottom of the
// pyramid" or "halfway up", which is what a decision about detail needs.
type Tile struct {
	Z       int
	X       int
	Y       int
	MaxZoom int
	Image   *image.RGBA
}

// Result describes the pyramid that was built. Width and Height are the
// decoded image's, after any EXIF orientation has been applied, which is what
// makes them the dimensions the pyramid is actually laid out over rather than
// the ones the file's header claims.
type Result struct {
	Width    int
	Height   int
	TileSize int
	MaxZoom  int
}

// Build decodes src and hands every tile of the pyramid to emit, level 0
// first. It returns once every tile has been emitted.
//
// emit is a callback rather than a returned slice because a 12000x9000 map is
// 584 tiles and a megabyte apiece, and nothing should hold all of that at once
// to hand it over afterwards. A caller that hands each tile to a worker pool
// gets backpressure for free: emit blocks when the pool is full, and the tiler
// stops running ahead of it. Returning an error from emit abandons the build
// and comes back out of Build unwrapped.
//
// PEAK MEMORY IS THE DECODED SOURCE PLUS LEVEL 1, which for a 108-megapixel
// PNG is around half a gigabyte. The source is only released once level 1 has
// been derived from it, and level 1 cannot be derived before level 0's tiles
// have been emitted, so the two overlap and there is no arrangement in which
// they do not. From level 2 on the numbers are quarters of quarters and stop
// mattering. It is the reason a caller should run one of these at a time.
func Build(src io.Reader, tileSize int, emit func(Tile) error) (Result, error) {
	if tileSize < 1 {
		return Result{}, fmt.Errorf("tiler: tile size %d is not a size", tileSize)
	}
	if emit == nil {
		return Result{}, errors.New("tiler: no emit function")
	}

	// AutoOrientation is why this is imaging rather than image.Decode: a
	// photograph of a hand-drawn map carries a rotation tag, and a pyramid
	// laid out over the unrotated pixels describes an image nobody will see.
	// Decoding happens once, here, and every level after level 0 comes from
	// the level below it rather than from another pass over this.
	img, err := imaging.Decode(src, imaging.AutoOrientation(true))
	if err != nil {
		return Result{}, fmt.Errorf("tiler: decode: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 1 || height < 1 {
		return Result{}, fmt.Errorf("tiler: decoded image is %dx%d", width, height)
	}

	result := Result{
		Width:    width,
		Height:   height,
		TileSize: tileSize,
		MaxZoom:  MaxZoom(width, height, tileSize),
	}

	level := img
	for z := 0; ; z++ {
		if err := emitLevel(level, result, z, emit); err != nil {
			return Result{}, err
		}
		if z == result.MaxZoom {
			return result, nil
		}
		level = halve(level, LevelPixels(width, z+1), LevelPixels(height, z+1))
	}
}

// emitLevel cuts one level into tiles and hands each of them to emit. The
// crop is a copy rather than a view because the tile outlives this call: the
// caller may still be encoding it when the level it came from is released.
func emitLevel(level image.Image, result Result, z int, emit func(Tile) error) error {
	origin := level.Bounds().Min
	across := LevelTiles(result.Width, result.TileSize, z)
	down := LevelTiles(result.Height, result.TileSize, z)

	for y := 0; y < down; y++ {
		for x := 0; x < across; x++ {
			tile := image.NewRGBA(image.Rect(
				0, 0,
				LevelTileSize(result.Width, result.TileSize, z, x),
				LevelTileSize(result.Height, result.TileSize, z, y),
			))
			draw.Draw(tile, tile.Bounds(), level, origin.Add(image.Pt(x*result.TileSize, y*result.TileSize)), draw.Src)

			if err := emit(Tile{Z: z, X: x, Y: y, MaxZoom: result.MaxZoom, Image: tile}); err != nil {
				return err
			}
		}
	}

	return nil
}

// halve derives the next level up from the one below it.
//
// AT EXACTLY 2:1, ApproxBiLinear IS THE FOUR-PIXEL BOX AVERAGE, which is the
// correct filter for this ratio: every destination pixel is the mean of the
// four source pixels it covers, so nothing aliases and nothing rings. Lanczos
// would be a worse filter here at a much higher price -- imaging's resize
// allocates an intermediate of destination width by source height whenever
// both axes change, which is 216 MB for the first halving of a 12000x9000 map.
// This allocates the destination and nothing else, and has fast paths for
// every type Go's decoders hand back.
//
// The destination is *image.RGBA both because the encoder wants that type and
// because it makes every level after the first one type instead of whatever
// the source file happened to decode to.
func halve(level image.Image, width int, height int) *image.RGBA {
	next := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(next, next.Bounds(), level, level.Bounds(), draw.Src, nil)
	return next
}
