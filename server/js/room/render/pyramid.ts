// A LINE-FOR-LINE PORT OF internal/tiler/pyramid.go, and it has to stay one.
//
// Three pieces of code decide the shape of a pyramid: the tiler that builds it,
// the route that validates a requested tile against it, and this, which works
// out which tiles a viewport covers. Two of them agreeing and the third
// disagreeing serves the wrong tile, or no tile, with nothing anywhere
// reporting an error -- so the Go file is the specification and pyramid.test.ts
// replays its worked example.
//
// LEVEL 0 IS NATIVE RESOLUTION AND z INCREASES AS DETAIL FALLS. That is the
// inverse of the slippy-map convention every web map uses, where zooming in
// raises z. It is the right way round here because a map is one image of a
// fixed size rather than a planet: the level that always exists is the one the
// pixels came at, and every level above it is derived. Mixing the two
// conventions renders a level off by however many they disagree by, which looks
// like a blurry or a cropped map rather than like an error.

// levelPixels returns the length of one axis at level z: ceil(size / 2^z).
//
// CEIL, NOT A BARE RIGHT SHIFT. A shift floors, and flooring 9000 five times
// gives 281 where the pixels say 282 -- a level one row short of its last row
// of pixels, which is a row of the map that no tile contains.
export function levelPixels(size: number, z: number): number {
	if (size <= 0) {
		return 0;
	}
	if (z >= 31) {
		return 1;
	}

	const step = 1 << z;
	if (step >= size) {
		return 1;
	}

	return (size + step - 1) >> z;
}

// levelTiles returns how many tiles cover one axis at level z. The last one is
// a remainder rather than a full tile whenever the axis does not divide evenly.
export function levelTiles(size: number, tileSize: number, z: number): number {
	if (tileSize < 1) {
		return 0;
	}

	return Math.ceil(levelPixels(size, z) / tileSize);
}

// levelTileSize returns the pixel size of the tile at index x of an axis at
// level z. Every tile is tileSize square except the last of a row or column,
// which is the remainder.
//
// EDGE TILES ARE NOT PADDED TO SQUARE. Padding would lay a transparent seam
// down two edges of every map that does not divide evenly, and the renderer
// knows the native dimensions and sizes the last row and column from them.
export function levelTileSize(size: number, tileSize: number, z: number, x: number): number {
	if (tileSize < 1 || x < 0 || x >= levelTiles(size, tileSize, z)) {
		return 0;
	}

	const remainder = levelPixels(size, z) - x * tileSize;

	return remainder < tileSize ? remainder : tileSize;
}

// maxZoom returns the top level of the pyramid: the smallest z at which both
// axes fit inside one tile, and so the level above which there is nothing left
// to halve.
export function maxZoom(width: number, height: number, tileSize: number): number {
	if (tileSize < 1) {
		return 0;
	}

	let z = 0;
	while (levelPixels(width, z) > tileSize || levelPixels(height, z) > tileSize) {
		z++;
	}

	return z;
}

// tileOrigin is where tile index x at level z starts, in NATIVE map pixels.
// Everything the renderer draws is in native pixels -- pawns, the grid, the
// pointer -- so a tile's rectangle is converted once, here, and no other module
// ever sees a z.
export function tileOrigin(tileSize: number, z: number, x: number): number {
	return x * tileSize * Math.pow(2, z);
}

// tileSpan is how many native pixels one whole tile covers at level z.
export function tileSpan(tileSize: number, z: number): number {
	return tileSize * Math.pow(2, z);
}
