package room

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

var (
	testTerrainID  = testID(1020)
	testTerrainAlt = testID(1021)
)

func pines() PictureInfo {
	return PictureInfo{
		Name:  "Pine forest",
		Image: "/assets/images/" + testTerrainID.String(),
		Width: 512, Height: 442,
	}
}
func hills() PictureInfo {
	return PictureInfo{
		Name:  "Rolling hills",
		Image: "/assets/images/" + testTerrainAlt.String(),
		Width: 512, Height: 512,
	}
}
func (w *world) addArt(asset ulid.ULID, info PictureInfo) ulid.ULID {
	w.t.Helper()
	lib := newLibrary()
	lib.pictures[pictureKey{id: asset, kind: PictureTerrain}] = info
	cmd := &PaletteAdd{Asset: asset}
	w.resolve(cmd, lib)
	before := len(w.s.Table.Palette)
	w.apply(cmd, w.gm)
	if len(w.s.Table.Palette) != before+1 {
		w.t.Fatalf("addArt: the palette went from %d entries to %d", before, len(w.s.Table.Palette))
	}
	return w.s.Table.Palette[len(w.s.Table.Palette)-1].ID
}
func (w *world) hexGrid() {
	w.t.Helper()
	g := w.s.Table.Grid
	g.Type = GridHexPointy
	w.apply(&TableSetGrid{Grid: g}, w.gm)
}
func (w *world) tileAt(layer ulid.ULID, q, r int) *Tile {
	for i := range w.s.Tiles {
		if w.s.Tiles[i].LayerID == layer && w.s.Tiles[i].Q == q && w.s.Tiles[i].R == r {
			return &w.s.Tiles[i]
		}
	}
	return nil
}
func TestAPaletteEntryIsReadOffTheTerrainShelf(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	entry := w.s.Table.Palette[0]
	if entry.ID != art {
		t.Fatal("the entry does not carry the id the palette was keyed by")
	}
	if entry.AssetID != testTerrainID {
		t.Error("the entry forgot which library asset it came from")
	}
	if entry.Name != "Pine forest" || entry.Image != pines().Image {
		t.Errorf("the entry is %+v, want the library's own name and picture", entry)
	}
}
func TestOnlyTerrainGoesInTheBag(t *testing.T) {
	w := newWorld(t)
	lib := newLibrary()
	lib.pictures[pictureKey{id: testTerrainID, kind: PictureToken}] = pines()
	w.refuseResolve(&PaletteAdd{Asset: testTerrainID}, lib, CodeNotFound)
}
func TestTheBagHoldsOnlySoManyPictures(t *testing.T) {
	w := newWorld(t)
	for i := range PaletteMax {
		w.addArt(testID(1100+i), pines())
	}
	lib := newLibrary()
	lib.pictures[pictureKey{id: testTerrainAlt, kind: PictureTerrain}] = hills()
	full := &PaletteAdd{Asset: testTerrainAlt}
	w.resolve(full, lib)
	if e := w.refuse(full, w.gm, CodeInvalid); !strings.Contains(e.Message, "24") {
		t.Errorf("the refusal does not say how many fit: %q", e.Message)
	}
}
func TestTheSamePictureGoesInTheBagOnce(t *testing.T) {
	w := newWorld(t)
	w.addArt(testTerrainID, pines())
	lib := newLibrary()
	lib.pictures[pictureKey{id: testTerrainID, kind: PictureTerrain}] = pines()
	again := &PaletteAdd{Asset: testTerrainID}
	w.resolve(again, lib)
	if e := w.refuse(again, w.gm, CodeInvalid); !strings.Contains(e.Message, "Pine forest") {
		t.Errorf("the refusal does not name what is already there: %q", e.Message)
	}
	if len(w.s.Table.Palette) != 1 {
		t.Fatalf("the bag holds %d entries, want 1", len(w.s.Table.Palette))
	}
}
func TestAStampReplacesWhateverHeldThatCell(t *testing.T) {
	w := newWorld(t)
	pine := w.addArt(testTerrainID, pines())
	hill := w.addArt(testTerrainAlt, hills())
	w.apply(&TilesStamp{Layer: w.layer, Art: pine, Cells: []Cell{{Q: 2, R: -1}}}, w.gm)
	w.apply(&TilesStamp{Layer: w.layer, Art: hill, Cells: []Cell{{Q: 2, R: -1}}}, w.gm)
	if len(w.s.Tiles) != 1 {
		t.Fatalf("that cell holds %d tiles, want 1", len(w.s.Tiles))
	}
	if got := w.tileAt(w.layer, 2, -1); got == nil || got.Art != hill {
		t.Error("the second stamp did not take the cell")
	}
}
func TestOneBatchNamingACellTwiceLeavesOneTile(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}, {Q: 1, R: 0}, {Q: 0, R: 0}}}, w.gm)
	if len(w.s.Tiles) != 2 {
		t.Fatalf("the batch left %d tiles, want 2", len(w.s.Tiles))
	}
}
func TestTheSameCellOnTwoFloorsIsTwoTiles(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 3, R: 3}}}, w.gm)
	w.apply(&TilesStamp{Layer: cellar, Art: art, Cells: []Cell{{Q: 3, R: 3}}}, w.gm)
	if len(w.s.Tiles) != 2 {
		t.Fatalf("two floors share %d tiles, want 2 -- a cell is identified by its floor too", len(w.s.Tiles))
	}
}
func TestTheTableHoldsOnlySoManyTiles(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	for i := range TilesMax {
		w.s.Tiles = append(w.s.Tiles, Tile{LayerID: w.layer, Art: art, Q: i, R: 0, By: testGMID})
	}
	e := w.refuse(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: -1, R: -1}}}, w.gm, CodeInvalid)
	if !strings.Contains(strings.ToLower(e.Message), "erase") {
		t.Errorf("the refusal does not say what to do about it: %q", e.Message)
	}
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
}
func TestABatchBiggerThanTheCapIsRefusedOutright(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	cells := make([]Cell, TileBatchMax+1)
	for i := range cells {
		cells[i] = Cell{Q: i, R: 0}
	}
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Cells: cells}, w.gm, CodeInvalid)
	w.refuse(&TilesErase{Layer: w.layer, Cells: cells}, w.gm, CodeInvalid)
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: cells[:TileBatchMax]}, w.gm)
	if len(w.s.Tiles) != TileBatchMax {
		t.Fatalf("a full batch laid %d tiles, want %d", len(w.s.Tiles), TileBatchMax)
	}
}
func TestAStampWithNoCellsIsRefused(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.refuse(&TilesStamp{Layer: w.layer, Art: art}, w.gm, CodeInvalid)
	w.refuse(&TilesErase{Layer: w.layer}, w.gm, CodeInvalid)
}
func TestAStampNamingArtThatIsNotInTheBagIsRefused(t *testing.T) {
	w := newWorld(t)
	w.addArt(testTerrainID, pines())
	w.refuse(&TilesStamp{Layer: w.layer, Art: testID(9999), Cells: []Cell{{Q: 0, R: 0}}}, w.gm, CodeNotFound)
}
func TestRemovingAPaletteEntryTakesItsTilesWithIt(t *testing.T) {
	w := newWorld(t)
	pine := w.addArt(testTerrainID, pines())
	hill := w.addArt(testTerrainAlt, hills())
	w.apply(&TilesStamp{Layer: w.layer, Art: pine, Cells: []Cell{{Q: 0, R: 0}, {Q: 1, R: 0}}}, w.gm)
	w.apply(&TilesStamp{Layer: w.layer, Art: hill, Cells: []Cell{{Q: 2, R: 0}}}, w.gm)
	w.apply(&PaletteRemove{Art: pine}, w.gm)
	if len(w.s.Table.Palette) != 1 || w.s.Table.Palette[0].ID != hill {
		t.Fatalf("the bag holds %d entries, want the hills alone", len(w.s.Table.Palette))
	}
	if len(w.s.Tiles) != 1 || w.s.Tiles[0].Art != hill {
		t.Fatalf("%d tiles survived, want the one stamped with the hills", len(w.s.Tiles))
	}
}
func TestRemovingArtNobodyHasIsNotFound(t *testing.T) {
	w := newWorld(t)
	w.refuse(&PaletteRemove{Art: testID(9999)}, w.gm, CodeNotFound)
}
func TestAStampTurnsByTheStepsItsCellsHave(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Rotation: 90, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Rotation: 60, Cells: []Cell{{Q: 1, R: 0}}}, w.gm, CodeInvalid)
	w.hexGrid()
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Rotation: 60, Cells: []Cell{{Q: 2, R: 0}}}, w.gm)
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Rotation: 90, Cells: []Cell{{Q: 3, R: 0}}}, w.gm, CodeInvalid)
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Rotation: -60, Cells: []Cell{{Q: 4, R: 0}}}, w.gm, CodeInvalid)
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Rotation: 360, Cells: []Cell{{Q: 5, R: 0}}}, w.gm, CodeInvalid)
}
func TestErasingIsByCellAndAStampRemembersWhoLaidIt(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}, {Q: 1, R: 0}}}, w.gm)
	laid := w.tileAt(w.layer, 0, 0)
	if laid == nil || laid.By != testGMID {
		t.Fatalf("the tile is %+v, want one that remembers the GM laid it", laid)
	}
	w.apply(&TilesErase{Layer: w.layer, Cells: []Cell{{Q: 0, R: 0}, {Q: 9, R: 9}}}, w.gm)
	if w.tileAt(w.layer, 0, 0) != nil {
		t.Error("the named cell still holds its tile")
	}
	if w.tileAt(w.layer, 1, 0) == nil {
		t.Error("erasing one cell took its neighbour too")
	}
}
func TestClearingTilesTakesOneFloorAndLeavesTheOthers(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.apply(&TilesStamp{Layer: cellar, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.apply(&TilesClear{Layer: w.layer}, w.gm)
	if w.tileAt(w.layer, 0, 0) != nil {
		t.Error("the cleared floor still holds a tile")
	}
	if w.tileAt(cellar, 0, 0) == nil {
		t.Error("clearing one floor took another floor's tiles")
	}
	if len(w.s.Table.Palette) != 1 {
		t.Error("clearing the tiles emptied the bag as well")
	}
}
func TestDeletingAFloorTakesItsTilesWithIt(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: cellar, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.apply(&TableRemoveLayer{Layer: cellar}, w.gm)
	if len(w.s.Tiles) != 1 || w.s.Tiles[0].LayerID != w.layer {
		t.Fatalf("%d tiles survived the floor, want the ground floor's alone", len(w.s.Tiles))
	}
}
func TestClearingTheTabletopTakesTheTilesAndLeavesTheBag(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	w.apply(&TableClear{}, w.gm)
	if len(w.s.Tiles) != 0 {
		t.Error("the tiles survived a cleared tabletop")
	}
	if len(w.s.Table.Palette) != 1 {
		t.Error("clearing the tabletop emptied the bag, which is a library and not a board")
	}
}
func TestForNowOnlyTheGMStamps(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.refuse(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.pc, CodeForbidden)
	w.refuse(&TilesErase{Layer: w.layer, Cells: []Cell{{Q: 0, R: 0}}}, w.pc, CodeForbidden)
	w.refuse(&TilesClear{Layer: w.layer}, w.pc, CodeForbidden)
	w.refuse(&PaletteAdd{Asset: testTerrainID}, w.pc, CodeForbidden)
	w.refuse(&PaletteRemove{Art: art}, w.pc, CodeForbidden)
}
func TestThePlayersSeeEveryTileTheGMSees(t *testing.T) {
	w := newWorld(t)
	art := w.addArt(testTerrainID, pines())
	w.apply(&TilesStamp{Layer: w.layer, Art: art, Cells: []Cell{{Q: 0, R: 0}}}, w.gm)
	p := w.s.Project(RolePlayer)
	if len(p.Tiles) != 1 || len(p.Table.Palette) != 1 {
		t.Fatalf("a player's copy holds %d tiles and %d palette entries, want 1 and 1", len(p.Tiles), len(p.Table.Palette))
	}
}
