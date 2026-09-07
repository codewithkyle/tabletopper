package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE TABLE FAMILY: the layers, the grid, and the two room-wide options. Every
// command here is the GM's, because every one of them changes what the whole
// table is looking at, and every one of them answers with the entire Table
// object rather than with the field that moved.
//
// THAT IS THE SINGLETON RULE and it is worth restating where it is used: the
// Table is a few hundred bytes and it changes when a person clicks a menu item,
// so there is nothing to win by sending a delta and a partial-update reducer to
// lose. The client assigns one object and is done.

// TableUpdated carries the whole table after any change to it.
type TableUpdated struct {
	Header
	Table Table `json:"table"`
}

func (*TableUpdated) eventType() string { return "table.updated" }

// tableUpdated is the emission nine of the commands below end with.
func tableUpdated(s *State) Emission {
	return to(ToAll, &TableUpdated{Table: cloneTable(s.Table)})
}

// TableAddLayer adds an empty floor above the ones already there.
type TableAddLayer struct {
	Name string `json:"name"`
}

func (c *TableAddLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "add a layer")
}

func (c *TableAddLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(s.Table.Layers) >= LayersMax {
		return nil, invalid("Too many layers", "A room can hold at most 20 layers.")
	}
	if err := checkRequiredName("layer", c.Name); err != nil {
		return nil, err
	}

	// The new layer inherits NewState's fog defaults rather than the layer
	// below it: fog off, prefill true. Inheriting would mean a GM who fogged
	// the ground floor gets a cellar that is already covered, which reads as
	// the app having done something they did not ask for.
	s.Table.Layers = append(s.Table.Layers, Layer{ID: env.id(), Name: c.Name, FogPrefill: true})
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// TableRemoveLayer deletes a floor and everything standing on it. The confirm
// modal in front of this one is where the GM is told the pawn count.
type TableRemoveLayer struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableRemoveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "remove a layer")
}

func (c *TableRemoveLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if len(s.Table.Layers) == 1 {
		return nil, invalid("Last layer", "A room needs at least one layer.")
	}

	// EVERY EMISSION FOR THE PAWNS IS BUILT BEFORE ANYTHING IS DELETED, because
	// each one needs to know whether the pawn was shown, and after the layer is
	// gone that question has no answer.
	var out []Emission
	var entriesChanged bool

	for _, p := range s.Pawns {
		if p.LayerID != c.Layer {
			continue
		}
		out = append(out, to(ToGM, &PawnRemoved{ID: p.ID}))
		if s.Shown(p) {
			out = append(out, to(ToPlayers, &PawnRemoved{ID: p.ID}))
		}
		if s.dropEntriesFor(p.ID) {
			entriesChanged = true
		}
	}

	s.Pawns = slices.DeleteFunc(s.Pawns, func(p Pawn) bool { return p.LayerID == c.Layer })
	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == c.Layer })
	s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == c.Layer })

	out = append(out,
		to(ToAll, &FogCleared{Layer: c.Layer}),
		to(ToAll, &StrokeCleared{Layer: c.Layer}),
	)
	if entriesChanged {
		out = append(out, initiativeUpdated(s)...)
	}

	// THE ACTIVE LAYER FALLS DOWNWARDS, to the layer below the one that went,
	// or upwards for the bottom one. A GM removing the floor they are standing
	// on means to be somewhere afterwards, and the nearest floor is the one
	// they were most recently thinking about.
	index := slices.IndexFunc(s.Table.Layers, func(l Layer) bool { return l.ID == c.Layer })
	wasActive := s.Table.ActiveLayer == c.Layer
	s.Table.Layers = slices.Delete(s.Table.Layers, index, index+1)
	if wasActive {
		s.Table.ActiveLayer = s.Table.Layers[max(index-1, 0)].ID
	}

	s.Normalize()

	out = append(out, tableUpdated(s))

	// The pawns on whatever became active were not shown a moment ago and are
	// now. Nothing was shown before this command that is still shown after, so
	// a plain sweep of the new active layer is the whole of it.
	if wasActive {
		out = append(out, s.shownTransitions(map[ulid.ULID]bool{})...)
	}

	return out, nil
}

// TableRenameLayer renames a floor.
type TableRenameLayer struct {
	Layer ulid.ULID `json:"layer"`
	Name  string    `json:"name"`
}

func (c *TableRenameLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "rename a layer")
}

func (c *TableRenameLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	if err := checkRequiredName("layer", c.Name); err != nil {
		return nil, err
	}

	l.Name = c.Name
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// TableMoveLayer reorders the stack. Index is the position the layer ends up
// at, counted from the bottom.
type TableMoveLayer struct {
	Layer ulid.ULID `json:"layer"`
	Index int       `json:"index"`
}

func (c *TableMoveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "reorder the layers")
}

func (c *TableMoveLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
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

	return []Emission{tableUpdated(s)}, nil
}

// TableSetLayerMap points a layer at a tiled map.
//
// IT IS A RESOLVED COMMAND. AssetID is what the browser sends and what the
// generated TypeScript declares; Map is filled in by the hub from the assets
// row before Apply runs, because the width, tile size and generation are
// database facts and this package does not have a database. An unresolved
// command is invalid rather than silently setting a map of zero by zero, so a
// hub that forgets to resolve fails loudly in a test.
type TableSetLayerMap struct {
	Layer   ulid.ULID `json:"layer"`
	AssetID ulid.ULID `json:"assetId"`

	Map *MapRef `json:"-"`
}

func (c *TableSetLayerMap) Authorize(s *State, a Actor) error {
	return requireGM(a, "change a layer's map")
}

func (c *TableSetLayerMap) Apply(s *State, a Actor, env Env) ([]Emission, error) {
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

	return []Emission{tableUpdated(s)}, nil
}

// TableClearLayerMap empties a layer back to blank. The pawns on it stay: a GM
// swapping the battle map under a fight has not deleted the fight.
type TableClearLayerMap struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableClearLayerMap) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear a layer's map")
}

func (c *TableClearLayerMap) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}

	l.Map = nil
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// TableSetActiveLayer changes what the players are looking at.
type TableSetActiveLayer struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *TableSetActiveLayer) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the active layer")
}

func (c *TableSetActiveLayer) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}

	before := s.shownSet()
	s.Table.ActiveLayer = c.Layer
	s.Normalize()

	// The table comes first, then the pawns. A client that received the pawns
	// first would spawn them onto a floor it still believes is the old one, and
	// the crossfade would run against the wrong map.
	out := []Emission{tableUpdated(s)}

	return append(out, s.shownTransitions(before)...), nil
}

// TableSetGrid replaces the grid whole.
type TableSetGrid struct {
	Grid Grid `json:"grid"`
}

func (c *TableSetGrid) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the grid")
}

func (c *TableSetGrid) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if err := checkGrid(c.Grid); err != nil {
		return nil, err
	}

	// PAWNS ARE NOT RE-SNAPPED. Changing the cell size under a placed encounter
	// and having every creature jump is worse than a grid that no longer lines
	// up with them, and the GM can drag whatever matters. The new grid applies
	// to the next thing that moves.
	s.Table.Grid = c.Grid
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// TableSetOptions carries both room-wide options at once, because they are one
// settings panel and sending the panel is the singleton rule again.
type TableSetOptions struct {
	MonsterHP      HPVisibility `json:"monsterHp"`
	PlayersCanDraw bool         `json:"playersCanDraw"`
}

func (c *TableSetOptions) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the table options")
}

func (c *TableSetOptions) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if !c.MonsterHP.Valid() {
		return nil, invalid("Bad setting", "That is not a hit point visibility setting.")
	}

	changed := s.Table.MonsterHP != c.MonsterHP
	s.Table.MonsterHP = c.MonsterHP
	s.Table.PlayersCanDraw = c.PlayersCanDraw
	s.Normalize()

	out := []Emission{tableUpdated(s)}
	if !changed {
		return out, nil
	}

	// THE PROJECTION OF EVERY MONSTER JUST CHANGED, and no pawn did. Nothing
	// else in the protocol would tell players that the goblin they are looking
	// at now has a number over it, so the setting emits the pawns itself.
	//
	// It is the shown pawns, not the visible ones: a pawn on another floor is
	// not in a player's state at all, and a pawn.updated for it would insert
	// one there.
	for _, p := range s.Pawns {
		if p.Kind != PawnMonster && p.Kind != PawnNPC {
			continue
		}
		if !s.Shown(p) {
			continue
		}
		out = append(out, to(ToPlayers, &PawnUpdated{Pawn: projectPawn(clonePawn(p), s.Table)}))
	}

	return out, nil
}
