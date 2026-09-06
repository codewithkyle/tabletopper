package tiler

// The pyramid's shape is entirely determined by three numbers -- the image's
// native width and height, and the tile size -- and these four functions are
// the only place that determination is written down. The tiler builds a
// pyramid from them, the tile route validates a requested z/x/y against them,
// and the renderer works out which tiles a viewport covers from them. Two of
// those three agreeing and the third disagreeing serves the wrong tile, or no
// tile, with nothing anywhere reporting an error.
//
// LEVEL 0 IS NATIVE RESOLUTION AND z INCREASES AS DETAIL FALLS. That is the
// inverse of the slippy-map convention every web map uses, where zooming in
// raises z. It is the right way round here because a map is one image of a
// fixed size rather than a planet, so the level that always exists is the one
// the pixels came at, and every level above it is derived. Mixing the two
// conventions renders a level that is off by however many the two disagree by,
// which looks like a blurry or a cropped map rather than like an error.

// LevelPixels returns the length of one axis at level z: ceil(size / 2^z).
//
// CEIL, NOT A BARE RIGHT SHIFT. A shift floors, and flooring 9000 five times
// gives 281 where the pixels say 282 -- a level one row short of its last row
// of pixels, which is a row of the map that no tile contains.
func LevelPixels(size int, z int) int {
	if size <= 0 {
		return 0
	}
	// A shift wider than the word is undefined-looking rather than useful,
	// and every level past the one that reaches a single pixel is that one
	// again. z arrives from a URL, so it is bounded here rather than trusted.
	if z >= 62 {
		return 1
	}
	step := 1 << uint(z)
	if step >= size {
		return 1
	}
	return (size + step - 1) >> uint(z)
}

// LevelTiles returns how many tiles cover one axis at level z. The last one is
// a remainder rather than a full tile whenever the axis does not divide evenly.
//
// A tileSize of zero or less is answered with zero tiles, which is what makes
// this safe to call from the route that validates a tile request: a row whose
// tile size is missing has no tile at any index rather than a division by zero.
func LevelTiles(size int, tileSize int, z int) int {
	if tileSize < 1 {
		return 0
	}
	pixels := LevelPixels(size, z)
	return (pixels + tileSize - 1) / tileSize
}

// LevelTileSize returns the pixel size of the tile at index x of an axis at
// level z. Every tile is tileSize square except the last of a row or column,
// which is the remainder.
//
// EDGE TILES ARE NOT PADDED TO SQUARE. Padding would lay a transparent or
// black seam down two edges of every map that does not divide evenly, and the
// renderer knows the native dimensions and can size the last row and column
// from them.
func LevelTileSize(size int, tileSize int, z int, x int) int {
	if tileSize < 1 || x < 0 || x >= LevelTiles(size, tileSize, z) {
		return 0
	}
	if remainder := LevelPixels(size, z) - x*tileSize; remainder < tileSize {
		return remainder
	}
	return tileSize
}

// MaxZoom returns the top level of the pyramid: the smallest z at which both
// axes fit inside one tile, and so the level above which there is nothing left
// to halve.
func MaxZoom(width int, height int, tileSize int) int {
	if tileSize < 1 {
		return 0
	}
	z := 0
	for LevelPixels(width, z) > tileSize || LevelPixels(height, z) > tileSize {
		z++
	}
	return z
}
