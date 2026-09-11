package tiler

import (
	"fmt"
	"testing"
)





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





func TestLevelPixelsCeilsRatherThanFloors(t *testing.T) {
	if got := LevelPixels(9000, 5); got != 282 {
		t.Errorf("LevelPixels(9000, 5) = %d, want 282", got)
	}
	if floored := 9000 >> 5; floored != 281 {
		t.Fatalf("the premise moved: 9000>>5 = %d", floored)
	}
}





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




func TestLevelTileSizeIsTheRemainderAtTheEdge(t *testing.T) {
	
	for x, want := range []int{8, 8, 1} {
		if got := LevelTileSize(17, 8, 0, x); got != want {
			t.Errorf("LevelTileSize(17, 8, 0, %d) = %d, want %d", x, got, want)
		}
	}
	
	for x, want := range []int{8, 8} {
		if got := LevelTileSize(16, 8, 0, x); got != want {
			t.Errorf("LevelTileSize(16, 8, 0, %d) = %d, want %d", x, got, want)
		}
	}
	if got := LevelTileSize(16, 8, 0, 2); got != 0 {
		t.Errorf("LevelTileSize past the last tile = %d, want 0", got)
	}
}





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

	
	
	for _, z := range []int{40, 62, 63, 1000} {
		if got := LevelPixels(12000, z); got != 1 {
			t.Errorf("LevelPixels(12000, %d) = %d, want 1", z, got)
		}
	}
	if got := MaxZoom(12000, 9000, 0); got != 0 {
		t.Errorf("MaxZoom with no tile size = %d, want 0", got)
	}
}
