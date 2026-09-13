package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

//go:generate go run ./gen

type State struct {
	Schema     int        `json:"schema"`
	Seq        uint64     `json:"seq"`
	Room       RoomInfo   `json:"room"`
	Table      Table      `json:"table"`
	Players    []Player   `json:"players"`
	Pawns      []Pawn     `json:"pawns"`
	Initiative Initiative `json:"initiative"`
	Fog        []FogShape `json:"fog"`
	Strokes    []Stroke   `json:"strokes"`
	Rolls      []Roll     `json:"rolls"`
	Music      Music      `json:"music"`
}
type RoomInfo struct {
	ID     ulid.ULID `json:"id"`
	Name   string    `json:"name"`
	Locked bool      `json:"locked"`
}
type Table struct {
	Layers []Layer `json:"layers"`
	TableSettings
}
type TableSettings struct {
	ActiveLayer        ulid.ULID          `json:"activeLayer"`
	Grid               Grid               `json:"grid"`
	PawnLabels         PawnLabels         `json:"pawnLabels"`
	PlayersCanDraw     bool               `json:"playersCanDraw"`
	InitiativeGrouping InitiativeGrouping `json:"initiativeGrouping"`
	FogPrefill         bool               `json:"fogPrefill"`
}
type Layer struct {
	ID         ulid.ULID `json:"id"`
	Name       string    `json:"name"`
	Map        *MapRef   `json:"map"`
	FogEnabled bool      `json:"fogEnabled"`
	FogPrefill bool      `json:"fogPrefill"`
	PartyStart *Point    `json:"partyStart"`
}
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type MapRef struct {
	AssetID  ulid.ULID `json:"assetId"`
	Gen      ulid.ULID `json:"gen"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	TileSize int       `json:"tileSize"`
	MaxZoom  int       `json:"maxZoom"`
}
type Grid struct {
	Lines       GridLines `json:"lines"`
	CellSize    int       `json:"cellSize"`
	OffsetX     int       `json:"offsetX"`
	OffsetY     int       `json:"offsetY"`
	Color       string    `json:"color"`
	Snap        Snap      `json:"snap"`
	FeetPerCell int       `json:"feetPerCell"`
	Diagonals   Diagonals `json:"diagonals"`
}
type Player struct {
	ID            ulid.ULID  `json:"id"`
	Name          string     `json:"name"`
	Avatar        string     `json:"avatar"`
	CharacterID   *ulid.ULID `json:"characterId"`
	CharacterName string     `json:"characterName"`
	Role          Role       `json:"role"`
	Connected     bool       `json:"connected"`
}

const DefaultAvatar = "/images/default-avatar.webp"

type Pawn struct {
	ID          ulid.ULID   `json:"id"`
	Kind        PawnKind    `json:"kind"`
	LayerID     ulid.ULID   `json:"layerId"`
	Name        string      `json:"name"`
	Image       string      `json:"image"`
	X           int         `json:"x"`
	Y           int         `json:"y"`
	Z           int         `json:"z"`
	Size        Size        `json:"size"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Rotation    int         `json:"rotation"`
	Visible     bool        `json:"visible"`
	HP          *int        `json:"hp"`
	MaxHP       *int        `json:"maxHp"`
	HPBand      *HPBand     `json:"hpBand"`
	AC          *int        `json:"ac"`
	Conditions  []Condition `json:"conditions"`
	OwnerID     *ulid.ULID  `json:"ownerId"`
	MonsterID   *ulid.ULID  `json:"monsterId"`
	CharacterID *ulid.ULID  `json:"characterId"`
}
type PawnPosition struct {
	ID ulid.ULID `json:"id"`
	X  int       `json:"x"`
	Y  int       `json:"y"`
}
type Condition struct {
	ID       ulid.ULID      `json:"id"`
	Name     string         `json:"name"`
	Color    ConditionColor `json:"color"`
	Duration int            `json:"duration"`
	Clear    ClearTrigger   `json:"clear"`
}
type Initiative struct {
	Entries []InitiativeEntry `json:"entries"`
	Active  *ulid.ULID        `json:"active"`
	Round   int               `json:"round"`
}
type InitiativeEntry struct {
	ID         ulid.ULID   `json:"id"`
	PawnIDs    []ulid.ULID `json:"pawnIds"`
	Name       string      `json:"name"`
	Initiative int         `json:"initiative"`
}
type InitiativeGrouping string

const (
	GroupMonsters   InitiativeGrouping = "grouped"
	GroupIndividual InitiativeGrouping = "individual"
)

func (InitiativeGrouping) Values() []string { return []string{"grouped", "individual"} }
func (g InitiativeGrouping) Valid() bool    { return inValues(g, g.Values()) }

type FogShape struct {
	ID      ulid.ULID `json:"id"`
	LayerID ulid.ULID `json:"layerId"`
	Kind    ShapeKind `json:"kind"`
	Mode    FogMode   `json:"mode"`
	Points  []int     `json:"points"`
}
type Stroke struct {
	ID      ulid.ULID  `json:"id"`
	By      ulid.ULID  `json:"by"`
	LayerID ulid.ULID  `json:"layerId"`
	Kind    StrokeKind `json:"kind"`
	Color   string     `json:"color"`
	Width   int        `json:"width"`
	Points  []int      `json:"points"`
	Done    bool       `json:"done"`
}
type GridLines string

const (
	GridLinesOff    GridLines = "off"
	GridLinesSolid  GridLines = "solid"
	GridLinesDashed GridLines = "dashed"
)

func (GridLines) Values() []string { return []string{"off", "solid", "dashed"} }
func (l GridLines) Valid() bool    { return inValues(l, l.Values()) }

type Snap string

const (
	SnapOff       Snap = "off"
	SnapCells     Snap = "cells"
	SnapHalfCells Snap = "halfCells"
)

func (Snap) Values() []string { return []string{"off", "cells", "halfCells"} }
func (s Snap) Valid() bool    { return inValues(s, s.Values()) }

type Diagonals string

const (
	DiagonalsEqual       Diagonals = "equal"
	DiagonalsAlternating Diagonals = "alternating"
)

func (Diagonals) Values() []string { return []string{"equal", "alternating"} }
func (d Diagonals) Valid() bool    { return inValues(d, d.Values()) }

type PawnLabels string

const (
	LabelsNone    PawnLabels = "none"
	LabelsDefault PawnLabels = "default"
	LabelsFull    PawnLabels = "full"
)

func (PawnLabels) Values() []string { return []string{"none", "default", "full"} }
func (v PawnLabels) Valid() bool    { return inValues(v, v.Values()) }
func ExactHP(kind PawnKind, labels PawnLabels, role Role) bool {
	if role == RoleGM {
		return true
	}
	if kind != PawnMonster && kind != PawnNPC {
		return true
	}
	return labels == LabelsFull
}

type PawnKind string

const (
	PawnPlayer  PawnKind = "player"
	PawnMonster PawnKind = "monster"
	PawnNPC     PawnKind = "npc"
	PawnObject  PawnKind = "object"
)

func (PawnKind) Values() []string { return []string{"player", "monster", "npc", "object"} }
func (k PawnKind) Valid() bool    { return inValues(k, k.Values()) }
func (k PawnKind) Creature() bool { return k != PawnObject }

type Size string

const (
	SizeTiny       Size = "tiny"
	SizeSmall      Size = "small"
	SizeMedium     Size = "medium"
	SizeLarge      Size = "large"
	SizeHuge       Size = "huge"
	SizeGargantuan Size = "gargantuan"
)

func (Size) Values() []string {
	return []string{"tiny", "small", "medium", "large", "huge", "gargantuan"}
}
func (s Size) Valid() bool { return inValues(s, s.Values()) }
func (s Size) Footprint() int {
	switch s {
	case SizeLarge:
		return 2
	case SizeHuge:
		return 3
	case SizeGargantuan:
		return 4
	default:
		return 1
	}
}

type HPBand string

const (
	BandHealthy    HPBand = "healthy"
	BandBruised    HPBand = "bruised"
	BandBloody     HPBand = "bloody"
	BandVeryBloody HPBand = "veryBloody"
	BandNearDeath  HPBand = "nearDeath"
	BandDead       HPBand = "dead"
)

func (HPBand) Values() []string {
	return []string{"healthy", "bruised", "bloody", "veryBloody", "nearDeath", "dead"}
}
func (b HPBand) Valid() bool { return inValues(b, b.Values()) }

type ConditionColor string

const (
	ColorBlue   ConditionColor = "blue"
	ColorGreen  ConditionColor = "green"
	ColorOrange ConditionColor = "orange"
	ColorPink   ConditionColor = "pink"
	ColorPurple ConditionColor = "purple"
	ColorRed    ConditionColor = "red"
	ColorWhite  ConditionColor = "white"
	ColorYellow ConditionColor = "yellow"
)

func (ConditionColor) Values() []string {
	return []string{"blue", "green", "orange", "pink", "purple", "red", "white", "yellow"}
}
func (c ConditionColor) Valid() bool { return inValues(c, c.Values()) }

type ClearTrigger string

const (
	ClearStart ClearTrigger = "start"
	ClearEnd   ClearTrigger = "end"
)

func (ClearTrigger) Values() []string { return []string{"start", "end"} }
func (c ClearTrigger) Valid() bool    { return inValues(c, c.Values()) }

type ShapeKind string

const (
	ShapeRect ShapeKind = "rect"
	ShapePoly ShapeKind = "poly"
)

func (ShapeKind) Values() []string { return []string{"rect", "poly"} }
func (k ShapeKind) Valid() bool    { return inValues(k, k.Values()) }

type StrokeKind string

const (
	StrokeFree   StrokeKind = "free"
	StrokeRect   StrokeKind = "rect"
	StrokeCircle StrokeKind = "circle"
	StrokeCone   StrokeKind = "cone"
)

func (StrokeKind) Values() []string { return []string{"free", "rect", "circle", "cone"} }
func (k StrokeKind) Valid() bool    { return inValues(k, k.Values()) }
func (k StrokeKind) Shape() bool    { return k != StrokeFree }

type FogMode string

const (
	FogReveal FogMode = "reveal"
	FogHide   FogMode = "hide"
)

func (FogMode) Values() []string { return []string{"reveal", "hide"} }
func (m FogMode) Valid() bool    { return inValues(m, m.Values()) }
func inValues[T ~string](v T, values []string) bool {
	return slices.Contains(values, string(v))
}

const DefaultLayerName = "Ground floor"

func NewState(roomID ulid.ULID, name string, env Env) *State {
	layer := Layer{
		ID:         env.id(),
		Name:       DefaultLayerName,
		FogEnabled: false,
		FogPrefill: true,
	}
	s := &State{
		Schema: Schema,
		Room:   RoomInfo{ID: roomID, Name: name},
		Table: Table{
			Layers: []Layer{layer},
			TableSettings: TableSettings{
				ActiveLayer: layer.ID,
				Grid: Grid{
					Lines:       GridLinesSolid,
					CellSize:    DefaultCellSize,
					Color:       DefaultGridColor,
					Snap:        SnapCells,
					FeetPerCell: DefaultFeetPerCell,
					Diagonals:   DiagonalsEqual,
				},
				PawnLabels:         LabelsDefault,
				PlayersCanDraw:     true,
				InitiativeGrouping: GroupMonsters,
			},
		},
	}
	s.Normalize()
	return s
}
func (s *State) Normalize() {
	s.Schema = Schema
	if !s.Table.Grid.Snap.Valid() {
		s.Table.Grid.Snap = SnapCells
	}
	if !s.Table.Grid.Lines.Valid() {
		s.Table.Grid.Lines = GridLinesSolid
	}
	if !s.Table.PawnLabels.Valid() {
		s.Table.PawnLabels = LabelsDefault
	}
	if !s.Table.InitiativeGrouping.Valid() {
		s.Table.InitiativeGrouping = GroupMonsters
	}
	if len(s.Initiative.Entries) == 0 {
		s.Initiative.Round = 0
	} else if s.Initiative.Round < 1 {
		s.Initiative.Round = 1
	}
	slices.SortFunc(s.Players, func(a, b Player) int { return a.ID.Compare(b.ID) })
	slices.SortFunc(s.Pawns, func(a, b Pawn) int { return a.ID.Compare(b.ID) })
	slices.SortFunc(s.Strokes, func(a, b Stroke) int { return a.ID.Compare(b.ID) })
	slices.SortFunc(s.Rolls, func(a, b Roll) int { return a.ID.Compare(b.ID) })
	s.Players = emptied(s.Players)
	s.Pawns = emptied(s.Pawns)
	s.Fog = emptied(s.Fog)
	s.Strokes = emptied(s.Strokes)
	s.Rolls = emptied(s.Rolls)
	s.Table.Layers = emptied(s.Table.Layers)
	s.Initiative.Entries = emptied(s.Initiative.Entries)
	for i := range s.Initiative.Entries {
		s.Initiative.Entries[i].PawnIDs = emptied(s.Initiative.Entries[i].PawnIDs)
	}
	for i := range s.Pawns {
		s.Pawns[i].Conditions = emptied(s.Pawns[i].Conditions)
	}
	for i := range s.Fog {
		s.Fog[i].Points = emptied(s.Fog[i].Points)
	}
	for i := range s.Strokes {
		s.Strokes[i].Points = emptied(s.Strokes[i].Points)
	}
	for i := range s.Rolls {
		s.Rolls[i].Dice = emptied(s.Rolls[i].Dice)
	}
}
func emptied[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
func (s *State) Layer(id ulid.ULID) *Layer {
	for i := range s.Table.Layers {
		if s.Table.Layers[i].ID == id {
			return &s.Table.Layers[i]
		}
	}
	return nil
}
func (s *State) Pawn(id ulid.ULID) *Pawn {
	for i := range s.Pawns {
		if s.Pawns[i].ID == id {
			return &s.Pawns[i]
		}
	}
	return nil
}
func (s *State) Player(id ulid.ULID) *Player {
	for i := range s.Players {
		if s.Players[i].ID == id {
			return &s.Players[i]
		}
	}
	return nil
}
func (s *State) Stroke(id ulid.ULID) *Stroke {
	for i := range s.Strokes {
		if s.Strokes[i].ID == id {
			return &s.Strokes[i]
		}
	}
	return nil
}
func (s *State) Shown(p Pawn) bool {
	return p.Visible && p.LayerID == s.Table.ActiveLayer
}
func (s *State) maxZ() int {
	z := 0
	for _, p := range s.Pawns {
		z = max(z, p.Z)
	}
	return z
}
func normalizeRotation(degrees int) int {
	d := degrees % 360
	if d < 0 {
		d += 360
	}
	return d
}
