





















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



export function levelTiles(size: number, tileSize: number, z: number): number {
	if (tileSize < 1) {
		return 0;
	}

	return Math.ceil(levelPixels(size, z) / tileSize);
}








export function levelTileSize(size: number, tileSize: number, z: number, x: number): number {
	if (tileSize < 1 || x < 0 || x >= levelTiles(size, tileSize, z)) {
		return 0;
	}

	const remainder = levelPixels(size, z) - x * tileSize;

	return remainder < tileSize ? remainder : tileSize;
}




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





export function tileOrigin(tileSize: number, z: number, x: number): number {
	return x * tileSize * Math.pow(2, z);
}


export function tileSpan(tileSize: number, z: number): number {
	return tileSize * Math.pow(2, z);
}
