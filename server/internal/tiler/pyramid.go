package tiler

func LevelPixels(size int, z int) int {
	if size <= 0 {
		return 0
	}
	if z >= 62 {
		return 1
	}
	step := 1 << uint(z)
	if step >= size {
		return 1
	}
	return (size + step - 1) >> uint(z)
}
func LevelTiles(size int, tileSize int, z int) int {
	if tileSize < 1 {
		return 0
	}
	pixels := LevelPixels(size, z)
	return (pixels + tileSize - 1) / tileSize
}
func LevelTileSize(size int, tileSize int, z int, x int) int {
	if tileSize < 1 || x < 0 || x >= LevelTiles(size, tileSize, z) {
		return 0
	}
	if remainder := LevelPixels(size, z) - x*tileSize; remainder < tileSize {
		return remainder
	}
	return tileSize
}
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
