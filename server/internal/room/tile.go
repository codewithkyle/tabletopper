package room

import (
	"context"
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

type TileArt struct {
	ID      ulid.ULID `json:"id"`
	AssetID ulid.ULID `json:"assetId"`
	Name    string    `json:"name"`
	Image   string    `json:"image"`
}
type Tile struct {
	LayerID  ulid.ULID `json:"layerId"`
	Art      ulid.ULID `json:"art"`
	Q        int       `json:"q"`
	R        int       `json:"r"`
	Rotation int       `json:"rotation"`
	By       ulid.ULID `json:"by"`
}
type Cell struct {
	Q int `json:"q"`
	R int `json:"r"`
}
type TilesStamped struct {
	Kind
	Tiles []Tile `json:"tiles"`
}

func (*TilesStamped) changeType() string { return "tiles.stamped" }

type TilesErased struct {
	Kind
	Layer ulid.ULID `json:"layer"`
	Cells []Cell    `json:"cells"`
}

func (*TilesErased) changeType() string { return "tiles.erased" }

type PaletteUpdated struct {
	Kind
	Palette []TileArt `json:"palette"`
}

func (*PaletteUpdated) changeType() string { return "palette.updated" }

func (s *State) Art(id ulid.ULID) *TileArt {
	for i := range s.Table.Palette {
		if s.Table.Palette[i].ID == id {
			return &s.Table.Palette[i]
		}
	}
	return nil
}
func (s *State) Tile(layer ulid.ULID, q, r int) *Tile {
	for i := range s.Tiles {
		if s.Tiles[i].LayerID == layer && s.Tiles[i].Q == q && s.Tiles[i].R == r {
			return &s.Tiles[i]
		}
	}
	return nil
}
func (s *State) requireArt(id ulid.ULID) (*TileArt, error) {
	art := s.Art(id)
	if art == nil {
		return nil, notFound("Terrain gone", "That terrain is no longer in the palette.")
	}
	return art, nil
}

type PaletteAdd struct {
	Asset ulid.ULID `json:"asset"`
	Art   *TileArt  `json:"-"`
}

func (c *PaletteAdd) Authorize(s *State, a Actor) error {
	return requireGM(a, "add terrain to the palette")
}
func (c *PaletteAdd) Resolve(ctx context.Context, lib Library, s *State) error {
	info, err := lib.Picture(ctx, c.Asset, PictureTerrain)
	if err != nil {
		return err
	}
	c.Art = &TileArt{AssetID: c.Asset, Name: info.Name, Image: info.Image}
	return nil
}
func (c *PaletteAdd) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if c.Art == nil {
		return nil, invalid("Terrain not read", "That terrain was never read out of the library.")
	}
	if held := s.artOf(c.Asset); held != nil {
		return nil, invalid("Already in the palette", held.Name+" is already in the palette.")
	}
	if len(s.Table.Palette) >= PaletteMax {
		return nil, invalid("Palette full", fmt.Sprintf("The palette holds %d pictures. Remove one before adding another.", PaletteMax))
	}
	art := *c.Art
	art.ID = env.id()
	s.Table.Palette = append(s.Table.Palette, art)
	s.Normalize()
	return nil, nil
}
func (s *State) artOf(asset ulid.ULID) *TileArt {
	for i := range s.Table.Palette {
		if s.Table.Palette[i].AssetID == asset {
			return &s.Table.Palette[i]
		}
	}
	return nil
}

type PaletteRemove struct {
	Art ulid.ULID `json:"art"`
}

func (c *PaletteRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove terrain from the palette")
}
func (c *PaletteRemove) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireArt(c.Art); err != nil {
		return nil, err
	}
	at := slices.IndexFunc(s.Table.Palette, func(art TileArt) bool { return art.ID == c.Art })
	s.Table.Palette = slices.Delete(s.Table.Palette, at, at+1)
	s.Tiles = slices.DeleteFunc(s.Tiles, func(t Tile) bool { return t.Art == c.Art })
	s.Normalize()
	return nil, nil
}

type TilesStamp struct {
	Layer    ulid.ULID `json:"layer"`
	Art      ulid.ULID `json:"art"`
	Rotation int       `json:"rotation"`
	Cells    []Cell    `json:"cells"`
}

func (c *TilesStamp) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}
	if err := s.requirePlayerLayer(a, c.Layer); err != nil {
		return err
	}
	if !s.Table.PlayersCanStamp {
		return forbidden("Stamping is off", "The GM has turned off stamping for players.")
	}
	return nil
}
func (c *TilesStamp) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	cells, err := checkedCells(c.Cells)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireArt(c.Art); err != nil {
		return nil, err
	}
	if err := checkTileRotation(s.Table.Grid, c.Rotation); err != nil {
		return nil, err
	}
	fresh := 0
	for _, cell := range cells {
		if s.Tile(c.Layer, cell.Q, cell.R) == nil {
			fresh++
		}
	}
	if len(s.Tiles)+fresh > TilesMax {
		return nil, invalid("Too many tiles", fmt.Sprintf("This room already holds %d tiles. Erase some before stamping more.", TilesMax))
	}
	for _, cell := range cells {
		if held := s.Tile(c.Layer, cell.Q, cell.R); held != nil {
			held.Art, held.Rotation, held.By = c.Art, c.Rotation, a.ID
			continue
		}
		s.Tiles = append(s.Tiles, Tile{
			LayerID:  c.Layer,
			Art:      c.Art,
			Q:        cell.Q,
			R:        cell.R,
			Rotation: c.Rotation,
			By:       a.ID,
		})
	}
	s.Normalize()
	return nil, nil
}

type TilesErase struct {
	Layer ulid.ULID `json:"layer"`
	Cells []Cell    `json:"cells"`
}

func (c *TilesErase) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}
	for _, cell := range c.Cells {
		if err := s.requireOwnTile(a, c.Layer, cell); err != nil {
			return err
		}
	}
	return nil
}
func (c *TilesErase) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	cells, err := checkedCells(c.Cells)
	if err != nil {
		return nil, err
	}
	s.Tiles = slices.DeleteFunc(s.Tiles, func(t Tile) bool {
		return t.LayerID == c.Layer && slices.Contains(cells, Cell{Q: t.Q, R: t.R})
	})
	s.Normalize()
	return nil, nil
}

type TilesClear struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TilesClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear a floor's terrain")
}
func (c *TilesClear) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	s.Tiles = slices.DeleteFunc(s.Tiles, func(t Tile) bool { return t.LayerID == c.Layer })
	s.Normalize()
	return nil, nil
}
func (s *State) requireOwnTile(a Actor, layer ulid.ULID, cell Cell) error {
	held := s.Tile(layer, cell.Q, cell.R)
	if held == nil {
		return notFound("Tile gone", "That cell no longer holds a tile.")
	}
	if held.By != a.ID {
		return forbidden("Not your tile", "You can only erase a tile you stamped.")
	}
	return nil
}
func checkedCells(cells []Cell) ([]Cell, error) {
	if len(cells) == 0 {
		return nil, invalid("No cells", "That names no cells to work on.")
	}
	if len(cells) > TileBatchMax {
		return nil, invalid("Too many cells", fmt.Sprintf("At most %d cells are stamped or erased at once.", TileBatchMax))
	}
	out := make([]Cell, 0, len(cells))
	for _, cell := range cells {
		if cell.Q < -CellLimit || cell.Q > CellLimit || cell.R < -CellLimit || cell.R > CellLimit {
			return nil, invalid("Off the map", fmt.Sprintf("A cell is within %d of the origin.", CellLimit))
		}
		if !slices.Contains(out, cell) {
			out = append(out, cell)
		}
	}
	return out, nil
}
func checkTileRotation(g Grid, degrees int) error {
	step := 90
	if g.Type.Hex() {
		step = 60
	}
	if degrees < 0 || degrees >= 360 || degrees%step != 0 {
		return invalid("Bad rotation", fmt.Sprintf("A tile turns in steps of %d degrees.", step))
	}
	return nil
}
