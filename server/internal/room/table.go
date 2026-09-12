package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

type TableUpdated struct {
	Header
	Table Table `json:"table"`
}

func (*TableUpdated) eventType() string { return "table.updated" }

type TableAddLayer struct {
	Name string `json:"name"`
}

func (c *TableAddLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "add a layer")
}
func (c *TableAddLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if len(s.Table.Layers) >= LayersMax {
		return nil, invalid("Too many layers", "A room can hold at most 20 layers.")
	}
	if err := checkRequiredName("layer", c.Name); err != nil {
		return nil, err
	}
	s.Table.Layers = append(s.Table.Layers, Layer{
		ID:         env.id(),
		Name:       c.Name,
		FogEnabled: s.Table.FogPrefill,
		FogPrefill: true,
	})
	s.Normalize()
	return nil, nil
}

type TableRemoveLayer struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableRemoveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove a layer")
}
func (c *TableRemoveLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if len(s.Table.Layers) == 1 {
		return nil, invalid("Last layer", "A room needs at least one layer.")
	}
	for _, p := range s.Pawns {
		if p.LayerID == c.Layer {
			s.dropEntriesFor(p.ID)
		}
	}
	s.Pawns = slices.DeleteFunc(s.Pawns, func(p Pawn) bool { return p.LayerID == c.Layer })
	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == c.Layer })
	s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == c.Layer })
	index := slices.IndexFunc(s.Table.Layers, func(l Layer) bool { return l.ID == c.Layer })
	wasActive := s.Table.ActiveLayer == c.Layer
	s.Table.Layers = slices.Delete(s.Table.Layers, index, index+1)
	if wasActive {
		s.Table.ActiveLayer = s.Table.Layers[max(index-1, 0)].ID
	}
	s.Normalize()
	return nil, nil
}

type TableRenameLayer struct {
	Layer ulid.ULID `json:"layer"`
	Name  string    `json:"name"`
}

func (c *TableRenameLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "rename a layer")
}
func (c *TableRenameLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	if err := checkRequiredName("layer", c.Name); err != nil {
		return nil, err
	}
	l.Name = c.Name
	s.Normalize()
	return nil, nil
}

type TableMoveLayer struct {
	Layer ulid.ULID `json:"layer"`
	Index int       `json:"index"`
}

func (c *TableMoveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "reorder the layers")
}
func (c *TableMoveLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	from := slices.IndexFunc(s.Table.Layers, func(l Layer) bool { return l.ID == c.Layer })
	if from < 0 {
		return nil, notFound("Layer gone", "That layer is no longer on the table.")
	}
	if c.Index < 0 || c.Index >= len(s.Table.Layers) {
		return nil, invalid("Bad position", "That is not a position in the layer list.")
	}
	l := s.Table.Layers[from]
	s.Table.Layers = slices.Delete(s.Table.Layers, from, from+1)
	s.Table.Layers = slices.Insert(s.Table.Layers, c.Index, l)
	s.Normalize()
	return nil, nil
}

type TableSetLayerMap struct {
	Layer   ulid.ULID `json:"layer"`
	AssetID ulid.ULID `json:"assetId"`
	Map     *MapRef   `json:"-"`
}

func (c *TableSetLayerMap) Authorize(s *State, a Actor) error {
	return requireGM(a, "change a layer's map")
}
func (c *TableSetLayerMap) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	if c.Map == nil {
		return nil, invalid("Map not ready", "That map could not be loaded.")
	}
	if c.Map.Width < 1 || c.Map.Height < 1 || c.Map.TileSize < 1 || c.Map.MaxZoom < 0 {
		return nil, invalid("Map not ready", "That map has not finished tiling.")
	}
	l.Map = cloneRef(c.Map)
	s.Normalize()
	return nil, nil
}

type TableClearLayerMap struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableClearLayerMap) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear a layer's map")
}
func (c *TableClearLayerMap) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	l.Map = nil
	s.Normalize()
	return nil, nil
}

type TableClear struct{}

func (c *TableClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the tabletop")
}
func (c *TableClear) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	for i := range s.Table.Layers {
		s.Table.Layers[i].Map = nil
	}
	s.Pawns = nil
	s.Fog = nil
	s.Strokes = nil
	s.Initiative = Initiative{}
	s.Normalize()
	return nil, nil
}

type TableSetActiveLayer struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableSetActiveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the active layer")
}
func (c *TableSetActiveLayer) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	s.Table.ActiveLayer = c.Layer
	s.Normalize()
	return nil, nil
}

type TableSetGrid struct {
	Grid Grid `json:"grid"`
}

func (c *TableSetGrid) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the grid")
}
func (c *TableSetGrid) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if err := checkGrid(c.Grid); err != nil {
		return nil, err
	}
	s.Table.Grid = c.Grid
	s.Normalize()
	return nil, nil
}

type TableSetOptions struct {
	PawnLabels         PawnLabels         `json:"pawnLabels"`
	PlayersCanDraw     bool               `json:"playersCanDraw"`
	InitiativeGrouping InitiativeGrouping `json:"initiativeGrouping"`
	FogPrefill         bool               `json:"fogPrefill"`
}

func (c *TableSetOptions) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the table options")
}
func (c *TableSetOptions) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if !c.PawnLabels.Valid() {
		return nil, invalid("Bad setting", "That is not a pawn label setting.")
	}
	if !c.InitiativeGrouping.Valid() {
		return nil, invalid("Bad setting", "That is not an initiative grouping.")
	}
	s.Table.PawnLabels = c.PawnLabels
	s.Table.PlayersCanDraw = c.PlayersCanDraw
	s.Table.InitiativeGrouping = c.InitiativeGrouping
	s.Table.FogPrefill = c.FogPrefill
	s.Normalize()
	return nil, nil
}
