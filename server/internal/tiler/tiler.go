package tiler

import (
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/disintegration/imaging"
	"golang.org/x/image/draw"
)

type Tile struct {
	Z       int
	X       int
	Y       int
	MaxZoom int
	Image   *image.RGBA
}
type Result struct {
	Width    int
	Height   int
	TileSize int
	MaxZoom  int
}

func Build(src io.Reader, tileSize int, emit func(Tile) error) (Result, error) {
	if tileSize < 1 {
		return Result{}, fmt.Errorf("tiler: tile size %d is not a size", tileSize)
	}
	if emit == nil {
		return Result{}, errors.New("tiler: no emit function")
	}
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
func halve(level image.Image, width int, height int) *image.RGBA {
	next := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(next, next.Bounds(), level, level.Bounds(), draw.Src, nil)
	return next
}
