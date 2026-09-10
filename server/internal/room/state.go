package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// The TypeScript mirror of everything in this file, plus every command and
// event, is generated rather than written. Run it with `go generate ./...` from
// ./server; TestProtocolTypesAreCurrent fails when the committed file is stale.
//
//go:generate go run ./gen

// State is one room, whole. It is the thing the room goroutine owns in phase 3,
// the thing that is marshalled into the snapshot column, and the thing a
// connecting client is sent a projection of. There is no second representation
// anywhere: the wire, the database and memory are the same shape.
//
// COORDINATES ARE INTEGERS IN MAP PIXELS, everywhere, without exception. Not
// cells, because a pawn between cells during a drag is an ordinary position
// rather than a special case; not floats, because two clients that round
// differently disagree about what they can see, and because integers are
// shorter as text on the one path that is measured in messages per second.
//
// IDS ARE ULIDs, which marshal through MarshalText as their 26-character
// string. That is why no type here carries custom JSON code: encoding/json
// finds the text marshaller on its own, on both sides, including inside a map
// key or a pointer.
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
}

// RoomInfo is the part of the rooms row the table needs to draw itself. The
// row is still the writer of record for all three: the HTTP lock route writes
// the database and the hub mirrors the change in here, never the other way
// round, so a reconnecting client and a fresh page load agree.
type RoomInfo struct {
	ID     ulid.ULID `json:"id"`
	Name   string    `json:"name"`
	Locked bool      `json:"locked"`
}

// Table is the room's configuration: what is under the pawns and how the pawns
// behave on it. It is a singleton in the sizing rules, which is to say that any
// change to any field of it broadcasts the whole object. It is a few hundred
// bytes and it changes when a person clicks a menu item, so there is nothing to
// win by sending less and a partial-update reducer to lose.
type Table struct {
	Layers      []Layer   `json:"layers"`
	ActiveLayer ulid.ULID `json:"activeLayer"`
	Grid        Grid      `json:"grid"`

	PawnLabels         PawnLabels         `json:"pawnLabels"`
	PlayersCanDraw     bool               `json:"playersCanDraw"`
	InitiativeGrouping InitiativeGrouping `json:"initiativeGrouping"`

	// FogPrefill is whether a floor ADDED FROM NOW ON starts covered, and it
	// is the whole of what the room-wide switch means.
	//
	// IT CHANGES NO FLOOR THAT ALREADY EXISTS, which is the difference between
	// a default and a master switch. A GM who has spent an evening uncovering
	// four floors and then reaches for this one must not lose that evening,
	// and the per-floor answer -- cover this one, uncover this one -- is two
	// items on the Fog menu rather than a setting.
	//
	// FALSE IS THE DEFAULT AND FALSE IS THE ZERO VALUE, which is why adding
	// this field needed no snapshot migration: every room written before it
	// existed reads back with the switch off, which is where it ships.
	FogPrefill bool `json:"fogPrefill"`
}

// Layer is a floor or a scene: a named slot holding at most one map, with its
// own fog flags. Only the active layer reaches players, and every pawn, fog
// shape and stroke names exactly one.
//
// THE GRID IS NOT HERE, deliberately. It is room-wide on the assumption that a
// building's floors were exported from the same source at the same scale, which
// is how every map pack in this genre ships. A layer whose map disagrees
// misaligns visibly rather than erroring, and the layer manager is where that
// gets pointed out.
type Layer struct {
	ID         ulid.ULID `json:"id"`
	Name       string    `json:"name"`
	Map        *MapRef   `json:"map"`
	FogEnabled bool      `json:"fogEnabled"`
	FogPrefill bool      `json:"fogPrefill"`
}

// MapRef is everything the renderer needs to fetch tiles, resolved once when
// the GM picks a map rather than looked up per client. Gen is the tiling
// generation from the assets row: it changes when a map is re-tiled, and it is
// in the tile URL, so a re-tile cannot be served from a stale cache.
type MapRef struct {
	AssetID  ulid.ULID `json:"assetId"`
	Gen      ulid.ULID `json:"gen"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	TileSize int       `json:"tileSize"`
	MaxZoom  int       `json:"maxZoom"`
}

// Grid is the room's one grid. OffsetX and OffsetY are unconstrained integers
// and the renderer reduces them modulo CellSize; storing the reduced value
// instead would make a GM nudging the offset past a cell boundary jump back to
// the other side of the cell, which is the opposite of what nudging means.
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

// Player is somebody at the table. The id is the user id, so a player who
// opens a second tab is one player with two connections, and Connected is a
// property of the person rather than of a socket.
//
// THE ROW SURVIVES A DISCONNECT. Losing wifi mid-combat must not delete the
// player, because their pawns are owned by that id and their initiative entry
// is named after it. Connected goes false, the row stays, and a reconnect is
// one field changing back.
//
// IT CARRIES TWO NAMES AND NEEDS BOTH. Name is the account -- who this is --
// and CharacterName is who they are playing, which is what a table calls them
// for the next four hours. The player list draws the character with the account
// in brackets after it, so the GM can tell two players apart when both of their
// characters are called Bob, and can tell which account to remove when one of
// them is not welcome.
//
// CHARACTERNAME IS A COPY AND NOT A LOOKUP. It is denormalised onto this row
// exactly as Name and Avatar are: the room is an actor holding its own state
// and cannot reach a database from inside its goroutine, so a name that had to
// be read per frame could not be sent at all. The socket resolves it once, when
// the connection opens, which is also when the character was chosen.
//
// It is empty for the GM, who brings no character, and empty for a player whose
// character was deleted out from under them. Both render as the account name
// alone, which is honest -- there is no character to name.
type Player struct {
	ID            ulid.ULID  `json:"id"`
	Name          string     `json:"name"`
	Avatar        string     `json:"avatar"`
	CharacterID   *ulid.ULID `json:"characterId"`
	CharacterName string     `json:"characterName"`
	Role          Role       `json:"role"`
	Connected     bool       `json:"connected"`
}

// DefaultAvatar is the picture an account with none of its own carries, and it
// is named here because Avatar above is the field that carries it.
//
// IT IS A PLACEHOLDER WEARING THE SHAPE OF A PICTURE, which is why anything
// that has a better placeholder of its own has to be able to tell. An <img> in
// the player list needs a URL that resolves, so the session and the users row
// both hold this rather than an empty string -- and a pawn on the canvas, which
// draws initials when it has no picture, must not mistake it for one. See
// characterPawn in internal/hub.
//
// IT IS ALSO THE COLUMN DEFAULT IN THE SCHEMA, so rows written before anything
// read this constant hold the same string.
const DefaultAvatar = "/images/default-avatar.webp"

// Pawn is anything on the table: a player's character, a monster from the
// manual, an unnamed npc, or an object like a wagon or a door.
//
// X AND Y ARE THE CENTRE OF THE FOOTPRINT, not a corner. Snapping, group moves
// and the movement path all reason about where a creature stands, and a corner
// is only the same thing as a position when everything is one cell.
//
// AN OBJECT IS MEASURED IN MAP PIXELS AND A CREATURE IN CELLS, which is Width
// and Height against Size. A creature's size is a category out of the rules --
// medium is one cell, large is two -- and it is the same square whatever the
// grid is set to. An object is a picture somebody drew at a size: a wagon is as
// wide as the wagon in the file, and forcing it onto a whole number of cells
// would letterbox every token that was not authored against this table's grid.
// So the two are different fields with different units rather than one field
// with a branch.
//
// AND AN OBJECT IS NOT ON THE LATTICE AT ALL. Nothing about a picture laid on a
// floor answers to cell parity: a rug is where somebody put it, a door sits in
// a wall rather than in a square, and a road runs at whatever angle the
// cartographer drew it. snapPawn returns an object's centre untouched, which is
// why Size -- the only thing that indexes the lattice -- is a creature's field
// and not a pawn's.
//
// ROTATION IS WHOLE DEGREES CLOCKWISE ABOUT THE CENTRE, and it is an object's
// alone. The centre is the origin because it is the one point that is still
// there after the turn -- an object rotated about a corner would walk away from
// where the GM put it -- and it is the same origin resizing uses, so the two
// gestures do not fight over where the thing IS. Every write folds it into
// [0, 360) so that nothing downstream has to know that -30 and 330 are one
// angle. A creature's is always zero: a disc has no facing, and turning the
// picture inside one would be a portrait leaning over in a circle whose edge
// nobody can see.
//
// WIDTH, HEIGHT AND ROTATION ARE ZERO FOR A CREATURE and Size is empty for an
// object. addPawn clears whichever set does not apply, so a pawn cannot be
// carrying a stale answer from before it was edited.
//
// HP, MAXHP, AC AND HPBAND ARE POINTERS because "unknown" and "zero" are
// different facts about a pawn and the player projection has to be able to say
// the first. A monster projected to players in a room labelling nothing has all
// four nil; in the default room it has HPBand set and the other three nil.
// HPBand is never set in the GM's copy: it exists only as a projection.
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

// PawnPosition is the payload of the two hot-path events. Moving eight pawns
// sends eight of these rather than eight whole pawns, and it is the only place
// in the protocol where an entity is represented by anything less than itself.
type PawnPosition struct {
	ID ulid.ULID `json:"id"`
	X  int       `json:"x"`
	Y  int       `json:"y"`
}

// Condition is a status on a pawn. The name is free text because a table
// invents conditions -- "on fire", "holding the rope" -- and a closed set would
// mean the twenty familiar ones and no others. The client offers those twenty
// as suggestions; the server takes any name that fits.
//
// DURATION IS IN TURNS AND -1 MEANS UNTIL REMOVED. Clear says which end of the
// pawn's own turn the count ticks on, which is the distinction 5e makes between
// a condition that lasts "until the start of your next turn" and one that lasts
// "until the end of it".
type Condition struct {
	ID       ulid.ULID      `json:"id"`
	Name     string         `json:"name"`
	Color    ConditionColor `json:"color"`
	Duration int            `json:"duration"`
	Clear    ClearTrigger   `json:"clear"`
}

// Initiative is the turn tracker. The slice order is the turn order -- there is
// no sort by the Initiative field, and the field is informational. A GM who
// wants two creatures to act in a particular order after a tie drags them, and
// a stored order is the only representation that can hold the result.
type Initiative struct {
	Entries []InitiativeEntry `json:"entries"`
	Active  *ulid.ULID        `json:"active"`
	Round   int               `json:"round"`
}

// InitiativeEntry is one line of the tracker.
//
// IT NAMES A LIST OF PAWNS AND NOT ONE PAWN, and the list is what makes grouped
// combat expressible at all. A solo line holds one id, the goblins hold nine,
// and a free-text line holds none -- which is how a lair action and "the
// volcano erupts" get a slot in the order without a pawn on the table.
//
// THE ALTERNATIVES WERE EACH TRIED ON PAPER. A REPRESENTATIVE PAWN -- one id,
// with the group re-derived around it -- makes the line lie the moment the
// representative dies, and turns the count into a query rather than a fact. A
// RENDER-TIME COLLAPSE -- thirteen lines drawn as three -- leaves the turn
// stepping through nine goblins one at a time behind a card that says nothing
// changed, which is the opposite of what grouped means. A PARALLEL GROUP TABLE
// is a second place for the same relationship to be wrong in.
//
// EVERY LOOP OVER IT IS A LOOP THAT ALREADY HAD A NIL CHECK IN IT, which is the
// whole cost of the change.
type InitiativeEntry struct {
	ID         ulid.ULID   `json:"id"`
	PawnIDs    []ulid.ULID `json:"pawnIds"`
	Name       string      `json:"name"`
	Initiative int         `json:"initiative"`
}

// InitiativeGrouping is how a sync turns monsters into lines of the tracker,
// and it is a table setting rather than a build-time choice because which kind
// of fight this is changes between fights.
//
// GROUPED IS THE DEFAULT BECAUSE IT IS HOW MOST TABLES RUN MOST FIGHTS. Nine
// goblins on nine counts is nine turns of bookkeeping for one decision.
// Individual is there because some fights deserve it, and a GM switches between
// them mid-session the way they switch any other table setting.
//
// IT APPLIES TO MONSTERS AND NOT TO NPCS. The kinds exist to draw that line: an
// NPC is a named individual -- the captain, the informant -- and grouping two of
// them under one line would be the app deciding they are interchangeable.
// Players are never grouped.
type InitiativeGrouping string

const (
	GroupMonsters   InitiativeGrouping = "grouped"
	GroupIndividual InitiativeGrouping = "individual"
)

func (InitiativeGrouping) Values() []string { return []string{"grouped", "individual"} }

func (g InitiativeGrouping) Valid() bool { return inValues(g, g.Values()) }

// FogShape is one rectangle or polygon on one layer. Shapes are the source of
// truth and the mask texture each client rasterises is a cache, which is what
// makes fog trivially undoable and tiny on the wire: a polygon is a few dozen
// integers where the mask it produces is a megabyte.
//
// ORDER IS MEANING HERE, unlike pawns. Shapes apply in slice order, so a hide
// drawn over a reveal covers it and the two cannot be sorted.
type FogShape struct {
	ID      ulid.ULID `json:"id"`
	LayerID ulid.ULID `json:"layerId"`
	Kind    ShapeKind `json:"kind"`
	Mode    FogMode   `json:"mode"`
	Points  []int     `json:"points"`
}

// Stroke is a drawn polyline. The client mints the id, because it references
// the stroke in chunks before the server has answered the first one; the server
// validates that the id parses and is unused, which is the whole of the trust
// it extends.
type Stroke struct {
	ID      ulid.ULID `json:"id"`
	By      ulid.ULID `json:"by"`
	LayerID ulid.ULID `json:"layerId"`
	Color   string    `json:"color"`
	Width   int       `json:"width"`
	Points  []int     `json:"points"`
	Done    bool      `json:"done"`
}

// GridLines is how the grid is drawn, and off is one of the ways. It is a
// single three-valued field rather than a visible flag beside a style because
// it is a single control: a GM asks "what do I want over this map", and the
// three answers are nothing, a line, and a dashed line.
//
// DASHED EXISTS FOR THE MAPS THAT ARE ALREADY DRAWN ON. A cartographer's floor
// has its own lines -- planks, flagstones, mortar -- and a solid grid laid over
// them competes with the picture, while a dashed one reads as an overlay and
// lets the floor through.
type GridLines string

const (
	GridLinesOff    GridLines = "off"
	GridLinesSolid  GridLines = "solid"
	GridLinesDashed GridLines = "dashed"
)

func (GridLines) Values() []string { return []string{"off", "solid", "dashed"} }
func (l GridLines) Valid() bool    { return inValues(l, l.Values()) }

// Snap is how a pawn's centre lands when it is dropped. See snap.go for the
// arithmetic; the two live modes are a whole-cell step and a half-cell one.
type Snap string

const (
	SnapOff       Snap = "off"
	SnapCells     Snap = "cells"
	SnapHalfCells Snap = "halfCells"
)

func (Snap) Values() []string { return []string{"off", "cells", "halfCells"} }
func (s Snap) Valid() bool    { return inValues(s, s.Values()) }

// Diagonals is the distance rule the movement path counts with: equal is 5e's
// default, where a diagonal costs the same as a straight step, and alternating
// is the 5-10-5 variant.
type Diagonals string

const (
	DiagonalsEqual       Diagonals = "equal"
	DiagonalsAlternating Diagonals = "alternating"
)

func (Diagonals) Values() []string { return []string{"equal", "alternating"} }
func (d Diagonals) Valid() bool    { return inValues(d, d.Values()) }

// PawnLabels is how much a pawn is described by, and to whom. It is a room
// setting rather than a per-pawn one because it is a table's style of play, not
// a property of a goblin.
//
// IT IS THREE POINTS ON ONE SCALE, which is why it is one control and not a
// pair of them. None tells nobody anything; default is the GM's table, where
// the GM reads numbers and the party reads words; full is an open table where
// everybody reads the same numbers. A fourth combination -- the GM told less
// than the players -- is not a way anybody runs a game.
//
// IT DECIDES WHAT IS SHOWN AND NOT WHAT IS SENT. Hit points go to everybody --
// see projectPawn, which argues it -- because the table cannot be DRAWN without
// them, and this then decides whether an interface prints them. Both readers
// are ExactHP below: pawnView in internal/controllers/room-pawns.go for the
// details window, and js/room/overlay.ts for the label under the pointer.
//
// IT STILL GOVERNS TEXT ONLY. A creature's blood, its pallor and its heartbeat
// are not labels -- they are the creature -- so they are drawn under all three
// settings, and none is a table with no words on it rather than a table where
// nobody bleeds. See wounds.ts, which is where that line is held.
//
// PLAYER-OWNED PAWNS AND OBJECTS ARE ALWAYS EXACT, whatever this says. A
// player's own character sheet is not a secret from the table, and a door's
// hit points are the thing the party is currently hitting.
type PawnLabels string

const (
	LabelsNone    PawnLabels = "none"
	LabelsDefault PawnLabels = "default"
	LabelsFull    PawnLabels = "full"
)

func (PawnLabels) Values() []string { return []string{"none", "default", "full"} }
func (v PawnLabels) Valid() bool    { return inValues(v, v.Values()) }

// ExactHP is whether an interface prints a pawn's hit points as numbers. It is
// the SHOWING rule and projectPawn is the SENDING one, and they are separate
// functions because they stopped being the same question: everybody is sent the
// numbers and only some viewers are shown them.
//
// A GM READS EVERYTHING, ALWAYS. There is no setting that hides a monster's hit
// points from the person running it, which is why the role is asked for here
// rather than inferred from what arrived -- a GM's copy is never projected and
// so never carries a band to be recognised by.
func ExactHP(kind PawnKind, labels PawnLabels, role Role) bool {
	if role == RoleGM {
		return true
	}
	if kind != PawnMonster && kind != PawnNPC {
		return true
	}

	return labels == LabelsFull
}

// PawnKind decides both what a pawn is drawn as and how it projects. Player
// pawns project unchanged; monsters and npcs go through the hit-point setting;
// objects are rectangles and carry no conditions.
type PawnKind string

const (
	PawnPlayer  PawnKind = "player"
	PawnMonster PawnKind = "monster"
	PawnNPC     PawnKind = "npc"
	PawnObject  PawnKind = "object"
)

func (PawnKind) Values() []string { return []string{"player", "monster", "npc", "object"} }
func (k PawnKind) Valid() bool    { return inValues(k, k.Values()) }

// Creature returns whether this kind has a size, conditions and hit points that
// mean anything. Everything that is not an object is a creature.
func (k PawnKind) Creature() bool { return k != PawnObject }

// Size is a creature's 5e size category. These are footprints, not the old
// client's drawing multipliers: a tiny creature occupies one cell and is drawn
// at half of it rather than occupying half a cell, because half a cell is not a
// position any grid rule can express.
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

// Footprint is how many cells on a side this size occupies: 1 for tiny through
// medium, then 2, 3 and 4. An unknown size answers 1 rather than 0, because a
// zero footprint would divide by nothing in the snapper.
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

// HPBand is the coarse health a player is shown instead of a number. It exists
// only in the player projection.
//
// SIX WORDS AND NOT A BAR, because a bar is a number drawn sideways: a party
// that can see a monster is at three fifths can work out its maximum from two
// hits, and then the fight is arithmetic again. A word is what somebody at a
// real table gets from looking, and the scale is deliberately coarse at the top
// -- everything above three quarters is one word -- and fine at the bottom,
// where the only question anyone is asking is whether one more hit will do it.
//
// DEAD IS SEPARATE FROM NEAR DEATH and that distinction is the point of having
// both. A creature at zero is down; a creature at one is the reason somebody
// spends their turn attacking rather than running.
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

// ConditionColor is the ring drawn around a pawn carrying the condition. It is
// a closed set of eight rather than a hex string because these are read at a
// glance across a shared screen, and a GM picking their own shade of brown is
// how a marker stops being legible.
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

// ClearTrigger is which end of a pawn's turn a condition's duration ticks on.
type ClearTrigger string

const (
	ClearStart ClearTrigger = "start"
	ClearEnd   ClearTrigger = "end"
)

func (ClearTrigger) Values() []string { return []string{"start", "end"} }
func (c ClearTrigger) Valid() bool    { return inValues(c, c.Values()) }

// ShapeKind is what a fog shape's points mean: a rect is two opposite corners
// and a poly is a closed ring.
type ShapeKind string

const (
	ShapeRect ShapeKind = "rect"
	ShapePoly ShapeKind = "poly"
)

func (ShapeKind) Values() []string { return []string{"rect", "poly"} }
func (k ShapeKind) Valid() bool    { return inValues(k, k.Values()) }

// FogMode is whether a shape uncovers the map or covers it. Which one a layer
// starts as is FogPrefill: a prefilled layer is covered and reveals are cut out
// of it, and an empty one is clear and hides are painted onto it.
type FogMode string

const (
	FogReveal FogMode = "reveal"
	FogHide   FogMode = "hide"
)

func (FogMode) Values() []string { return []string{"reveal", "hide"} }
func (m FogMode) Valid() bool    { return inValues(m, m.Values()) }

// inValues is the one membership check behind every Valid method above. Each
// enum's Valid is written in terms of its own Values so the generator's literal
// union and the server's validation cannot come apart: there is one list.
func inValues[T ~string](v T, values []string) bool {
	return slices.Contains(values, string(v))
}

// DefaultLayerName is what the one layer a new room starts with is called. It
// is a name rather than "Layer 1" because the case this feature exists for is a
// building with floors, and a GM who never adds a second layer never has to
// think about the first one having a name at all.
const DefaultLayerName = "Ground floor"

// NewState is the room a GM gets when they open one for the first time, and the
// room the hub falls back to when a snapshot is missing or unreadable.
//
// IT TAKES AN ENV, which the plan's sketch of this signature did not, because
// the one layer it creates needs an id and every other id in this package comes
// from the same place. A test that mints ids in sequence gets a deterministic
// starting state out of this, which is what makes the golden fixtures diffable.
func NewState(roomID ulid.ULID, name string, env Env) *State {
	layer := Layer{
		ID:   env.id(),
		Name: DefaultLayerName,

		// FOG STARTS OFF AND PREFILLED, which is not a contradiction: off
		// means nothing is drawn over the map, and prefilled is what happens
		// the moment it is switched on. A GM turning fog on means "hide this
		// map and let me reveal it", so the flag that describes that is the
		// one that should already be set when they reach for the switch.
		FogEnabled: false,
		FogPrefill: true,
	}

	s := &State{
		Schema: Schema,
		Room:   RoomInfo{ID: roomID, Name: name},
		Table: Table{
			Layers:      []Layer{layer},
			ActiveLayer: layer.ID,
			Grid: Grid{
				Lines:       GridLinesSolid,
				CellSize:    DefaultCellSize,
				Color:       DefaultGridColor,
				Snap:        SnapCells,
				FeetPerCell: DefaultFeetPerCell,
				Diagonals:   DiagonalsEqual,
			},

			// The middle one, and it is the middle one for the reason the
			// other two are extremes. Full turns every fight into arithmetic
			// about when to run; none means a player cannot tell a scratch
			// from a killing blow and stops describing what they did. Bloody
			// is the word the table already uses.
			PawnLabels: LabelsDefault,

			// Players draw by default because the drawing tool's ordinary use
			// is a player sketching the plan on the tavern table, and a GM who
			// does not want that turns it off once.
			PlayersCanDraw: true,

			// And monsters are grouped by default; see InitiativeGrouping.
			InitiativeGrouping: GroupMonsters,
		},
	}

	s.Normalize()

	return s
}

// Normalize puts the state in its one canonical form, and every Apply calls it
// before returning. Two states that hold the same facts come out of it
// byte-identical when marshalled, which is the property the golden fixtures and
// the convergence test are both written against -- without it, "the same state"
// would be a comparison somebody had to write and keep correct per type.
//
// IT SORTS THE THREE COLLECTIONS WHOSE ORDER IS NOT MEANING. Players, pawns and
// strokes are looked up by id and drawn by their own rules -- pawns by Z,
// strokes by the order they were begun in, which their ULIDs already encode --
// so slice order carries nothing and sorting it costs nothing.
//
// IT LEAVES FOG AND INITIATIVE ALONE, because in both of those the order IS the
// meaning. A hide drawn over a reveal covers it, and the turn order is the
// order the GM dragged the lines into.
//
// IT ALSO REPLACES NIL SLICES WITH EMPTY ONES. A nil slice marshals as null,
// and the generated TypeScript declares these fields as arrays; a reducer that
// has to check for null on every collection is a reducer with a bug waiting in
// whichever branch nobody wrote.
func (s *State) Normalize() {
	s.Schema = Schema

	// A SNAPPING MODE THAT NO LONGER EXISTS BECOMES THE DEFAULT. The set has
	// been narrowed once already -- "corners" was the parity rule inverted and
	// is gone -- and a room snapshot written before that is read back with a
	// value nothing accepts: the grid form shows no radio selected, and the
	// first thing the GM changes is refused for a field they never touched.
	// The room is otherwise intact and is not worth throwing away over this,
	// so the one field is repaired and the rest of the table stands.
	if !s.Table.Grid.Snap.Valid() {
		s.Table.Grid.Snap = SnapCells
	}

	// AND SO DOES A GRID WITH NO LINE STYLE, which is every snapshot written
	// before the field existed. Those rooms carried a visible flag instead, and
	// solid is what a true one was drawn as -- so the repair is right for the
	// tables that had a grid and wrong only for the few that had turned it off,
	// who see it come back once and switch it off again. That is the whole cost
	// of not bumping the schema, which would have thrown every pawn on every
	// table away to save them the one click.
	if !s.Table.Grid.Lines.Valid() {
		s.Table.Grid.Lines = GridLinesSolid
	}

	// AND A LABEL SETTING NOTHING ACCEPTS BECOMES THE DEFAULT, which is every
	// snapshot written while the field was called monsterHp and held hidden,
	// band or exact. Default is band's successor and is what the great majority
	// of those rooms were on; a room that had been set to either extreme comes
	// back in the middle once and the GM sets it again. The alternative is
	// bumping the schema, which throws every pawn on every table away.
	if !s.Table.PawnLabels.Valid() {
		s.Table.PawnLabels = LabelsDefault
	}

	// AND A ROOM WRITTEN BEFORE THERE WAS A GROUPING SETTING GETS THE DEFAULT,
	// which is every snapshot older than this. It is the same repair as the two
	// above and it is the cheap one: the setting is read when the GM presses
	// Sync and changes no projection, so a room that comes back grouped is a
	// room whose next sync groups.
	if !s.Table.InitiativeGrouping.Valid() {
		s.Table.InitiativeGrouping = GroupMonsters
	}

	// A TRACKER WITH LINES IN IT IS IN ROUND ONE AT THE EARLIEST, and an empty
	// one is in no round at all.
	//
	// THE FIGHT BEGINS WHEN THE ORDER DOES, not when somebody presses the
	// button. Sync built the order and everybody at the table is in round one
	// from that moment: a GM who gives the turn to whoever rolled highest by
	// clicking their card, rather than by pressing Next, was walking a whole
	// lap with the counter reading nothing before it turned over to 1 -- and
	// the number is what the party's spell durations are counted in.
	//
	// IT IS HERE RATHER THAN IN THE FOUR COMMANDS because it is a fact about
	// the shape of the state and not about any one of them, and because a
	// snapshot written before this rule existed comes back repaired for free.
	// The empty half of it is dropMembers' as well, which is the same rule
	// arriving from the other direction.
	if len(s.Initiative.Entries) == 0 {
		s.Initiative.Round = 0
	} else if s.Initiative.Round < 1 {
		s.Initiative.Round = 1
	}

	slices.SortFunc(s.Players, func(a, b Player) int { return a.ID.Compare(b.ID) })
	slices.SortFunc(s.Pawns, func(a, b Pawn) int { return a.ID.Compare(b.ID) })
	slices.SortFunc(s.Strokes, func(a, b Stroke) int { return a.ID.Compare(b.ID) })

	s.Players = emptied(s.Players)
	s.Pawns = emptied(s.Pawns)
	s.Fog = emptied(s.Fog)
	s.Strokes = emptied(s.Strokes)
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
}

// emptied is the nil-to-empty half of Normalize, in one place so that adding a
// collection to State is one line rather than one line and a forgotten one.
func emptied[T any](v []T) []T {
	if v == nil {
		return []T{}
	}

	return v
}

// Layer finds a layer by id. Callers that are about to mutate take the pointer;
// the slice is the storage, so the pointer is the layer.
func (s *State) Layer(id ulid.ULID) *Layer {
	for i := range s.Table.Layers {
		if s.Table.Layers[i].ID == id {
			return &s.Table.Layers[i]
		}
	}

	return nil
}

// Pawn finds a pawn by id.
func (s *State) Pawn(id ulid.ULID) *Pawn {
	for i := range s.Pawns {
		if s.Pawns[i].ID == id {
			return &s.Pawns[i]
		}
	}

	return nil
}

// Player finds a player by id.
func (s *State) Player(id ulid.ULID) *Player {
	for i := range s.Players {
		if s.Players[i].ID == id {
			return &s.Players[i]
		}
	}

	return nil
}

// Stroke finds a stroke by id.
func (s *State) Stroke(id ulid.ULID) *Stroke {
	for i := range s.Strokes {
		if s.Strokes[i].ID == id {
			return &s.Strokes[i]
		}
	}

	return nil
}

// Shown is the single predicate every player-facing pawn decision is written
// against: a pawn reaches players when it is visible and on the layer they are
// looking at. It is one function and not two conditions spelled out at each
// call site, because there are eleven call sites and the bug this design exists
// to prevent is exactly one of them disagreeing.
func (s *State) Shown(p Pawn) bool {
	return p.Visible && p.LayerID == s.Table.ActiveLayer
}

// maxZ is the top of the draw order, so a freshly spawned pawn lands on top of
// what is already there rather than under it.
func (s *State) maxZ() int {
	z := 0
	for _, p := range s.Pawns {
		z = max(z, p.Z)
	}

	return z
}

// normalizeRotation folds an angle into [0, 360).
//
// IT NORMALIZES RATHER THAN REFUSES, which is the difference between an angle
// and every other number in this package. -30 and 330 are the same facing, so
// rejecting one of them would be rejecting a way of writing the other; there is
// no such thing as an out-of-range rotation, only one that has not been reduced
// yet. Go's % keeps the sign of the dividend, which is the whole reason this is
// three lines rather than one.
func normalizeRotation(degrees int) int {
	d := degrees % 360
	if d < 0 {
		d += 360
	}

	return d
}
