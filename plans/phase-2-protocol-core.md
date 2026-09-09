# Phase 2: the protocol core, pure Go

Read `plans/vtt-overview.md` first, then `plans/phase-1-rooms.md` for what
already exists (`internal/room/code.go`, `room.Role`). This phase adds no
routes, no socket, no migration and no JavaScript beyond one generated types
file that nothing imports yet. It is the executable form of the overview's
Protocol design section, and it is verifiable entirely with `go test`.

## As built, 2026-09-07

Everything in the sections below shipped. Five things differ from what they
sketch, each for a reason the code carries in a comment:

- **`NewState(roomID, name, env)`** takes an `Env`. The one layer it creates
  needs an id, and every other id in the package comes from the same injected
  place; without it a golden fixture would be a regeneration rather than a diff.
- **`Env` has a second field, `Version`.** The snapshot event carries the server
  build so a client with a stale bundle reloads, and `sync.request` is the only
  thing that reads it.
- **`Authorize` may also answer `not_found`.** "May you move pawn X" has no
  answer when X is not there, and answering `forbidden` would tell a player
  their own pawn belongs to somebody else.
- **`ForRole` may answer nil**, meaning "this role is told nothing". It is what
  suppresses a `pawn.moved` whose every pawn is hidden. The projection itself
  happens in `Apply`, which has the state; only the three events sent to an
  audience spanning both roles carry two copies of themselves.
- **The generator writes a `.ts`, not a `.d.ts`**, because `TRANSIENT_EVENTS` is
  a runtime value and a declaration file cannot hold one. `make protocol` runs
  it; `TestProtocolTypesAreCurrent` fails when the committed file is stale.

Two things this phase found and did not decide, both recorded under "Still open"
in the overview: how `seq` is assigned across two audiences that receive
different numbers of events, and that players receive every layer's name and map
reference rather than only the active one.

## End state

- `server/internal/room` holds the room state types, every command and event in
  the catalog, authorization and application logic, per-audience emission,
  snapshot encode and decode, and a reference reducer.
- `go generate ./internal/room/...` writes `server/js/room/protocol.ts`, the
  TypeScript types for every command, event and state entity, including the
  two discriminated unions. A test fails when the committed file is stale.
- The convergence property from the overview is a test: for both audiences,
  applying the emitted events to the projected before-state with the reference
  reducer yields the projected after-state.
- Golden fixtures under `internal/room/testdata/reducer/` hold event sequences
  and expected states for the TypeScript reducer to replay in phase 3.

Nothing here touches the database or the network. A command that needs a
database lookup (set map, spawn from a monster or character) is defined here
with its fields already resolved; phase 3's hub does the lookup and builds it.

## Package layout

```
server/internal/room/
  code.go            phase 1: room codes
  role.go            phase 1: Role, RoleGM, RolePlayer
  state.go           State and entity types, enums, canonical ordering
  snapshot.go        Schema const, Marshal, Unmarshal, Project
  command.go         Command, Actor, Env, Emission, Audience, Error, DecodeCommand, registry
  event.go           Event, Header, EncodeEvent, registry
  table.go           table.* commands and events
  pawn.go            pawn.* commands and events
  initiative.go      initiative.* commands and events
  fog.go             fog.* commands and events
  stroke.go          stroke.* commands and events
  player.go          player.* commands and events (hub-only commands live here too)
  ping.go            ping command and pinged event
  sync.go            sync.request command, snapshot event
  snap.go            grid snapping
  validate.go        bounds, limits, helpers
  reduce.go          Reduce(*State, Event) error: the reference reducer
  gen/main.go        the TypeScript generator, run by go:generate
  testdata/reducer/  golden fixtures written by a test, committed
```

Everything is one package. Files are split by family so a reader finds a
command's type, its Authorize, its Apply and its events together.

## State

All coordinates are integers in map pixels. All ids are `ulid.ULID`, which
marshals as its 26-character string through `MarshalText`; no custom JSON code
is needed. JSON keys are camelCase via struct tags.

```go
type State struct {
    Schema     int          `json:"schema"`
    Seq        uint64       `json:"seq"`
    Room       RoomInfo     `json:"room"`
    Table      Table        `json:"table"`
    Players    []Player     `json:"players"`
    Pawns      []Pawn       `json:"pawns"`
    Initiative Initiative   `json:"initiative"`
    Fog        []FogShape   `json:"fog"`        // every layer's shapes, in application order
    Strokes    []Stroke     `json:"strokes"`
}

type RoomInfo struct {
    ID     ulid.ULID `json:"id"`
    Name   string    `json:"name"`
    Locked bool      `json:"locked"`
}

type Table struct {
    Layers         []Layer      `json:"layers"`          // ordered bottom to top; never empty
    ActiveLayer    ulid.ULID    `json:"activeLayer"`     // what players see; GM-controlled
    Grid           Grid         `json:"grid"`            // room-wide, shared by every layer
    PawnLabels     PawnLabels   `json:"pawnLabels"`      // none | default | full
    PlayersCanDraw bool         `json:"playersCanDraw"`
}

type Layer struct {
    ID         ulid.ULID `json:"id"`
    Name       string    `json:"name"`        // "Ground floor", "Cellar"
    Map        *MapRef   `json:"map"`         // nil: a blank layer
    FogEnabled bool      `json:"fogEnabled"`
    FogPrefill bool      `json:"fogPrefill"`  // true: the layer starts covered and shapes reveal
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
    Lines       GridLines `json:"lines"`        // off | solid | dashed, default solid
    CellSize    int       `json:"cellSize"`     // px, 8..512, default 64
    OffsetX     int       `json:"offsetX"`      // px, any int; renderer reduces modulo CellSize
    OffsetY     int       `json:"offsetY"`
    Color       string    `json:"color"`        // #RRGGBB or #RRGGBBAA, default #000000FF
    Snap        Snap      `json:"snap"`         // off | cells | corners, default cells
    FeetPerCell int       `json:"feetPerCell"`  // default 5
    Diagonals   Diagonals `json:"diagonals"`    // equal | alternating, default equal
}

type Player struct {
    ID          ulid.ULID  `json:"id"`          // the user id
    Name        string     `json:"name"`        // session username
    Avatar      string     `json:"avatar"`      // session profile image URL
    CharacterID *ulid.ULID `json:"characterId"`
    Role        Role       `json:"role"`
    Connected   bool       `json:"connected"`
}

type Pawn struct {
    ID          ulid.ULID   `json:"id"`
    Kind        PawnKind    `json:"kind"`        // player | monster | npc | object
    LayerID     ulid.ULID   `json:"layerId"`
    Name        string      `json:"name"`
    Image       string      `json:"image"`       // URL path or absolute URL; "" draws a disc with initials
    X           int         `json:"x"`           // centre of the footprint
    Y           int         `json:"y"`
    Z           int         `json:"z"`           // draw order, higher on top
    Size        Size        `json:"size"`        // creatures; ignored for object
    FootprintW  int         `json:"footprintW"`  // object only, cells wide, 1..20
    FootprintH  int         `json:"footprintH"`  // object only, cells tall, 1..20
    Visible     bool        `json:"visible"`
    HP          *int        `json:"hp"`          // nil when projected away
    MaxHP       *int        `json:"maxHp"`
    HPBand      *HPBand     `json:"hpBand"`      // set only in the player projection, on the default setting
    AC          *int        `json:"ac"`
    Conditions  []Condition `json:"conditions"`
    OwnerID     *ulid.ULID  `json:"ownerId"`     // player id; nil for GM-owned
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
    Name     string         `json:"name"`      // free text, 64 runes; the client offers the twenty familiar ones
    Color    ConditionColor `json:"color"`     // blue green orange pink purple red white yellow
    Duration int            `json:"duration"`  // turns remaining; -1 means until removed
    Clear    ClearTrigger   `json:"clear"`     // start | end (of the pawn's turn)
}

type Initiative struct {
    Entries []InitiativeEntry `json:"entries"`  // stored order is turn order
    Active  *ulid.ULID        `json:"active"`   // entry id
    Round   int               `json:"round"`    // 0 until the first next
}

type InitiativeEntry struct {
    ID         ulid.ULID  `json:"id"`
    PawnID     *ulid.ULID `json:"pawnId"`       // nil for a free-text entry such as a lair action
    Name       string     `json:"name"`
    Initiative int        `json:"initiative"`   // informational; order is the slice
}

type FogShape struct {
    ID      ulid.ULID `json:"id"`
    LayerID ulid.ULID `json:"layerId"`
    Kind    ShapeKind `json:"kind"`             // rect | poly
    Mode    FogMode   `json:"mode"`             // reveal | hide
    Points  []int     `json:"points"`           // flat x,y pairs; rect has exactly 4 numbers
}

type Stroke struct {
    ID     ulid.ULID `json:"id"`
    By     ulid.ULID `json:"by"`
    LayerID ulid.ULID `json:"layerId"`
    Color  string    `json:"color"`
    Width  int       `json:"width"`             // 1..64 px in map space
    Points []int     `json:"points"`            // flat x,y pairs
    Done   bool      `json:"done"`
}
```

Enums are named string types. Each one has a `Values() []string` method on
the type and a `Valid() bool` that checks membership. `Values` is what the
TypeScript generator reads to emit a literal union, and what validation reads,
so the two cannot disagree. Enum types: `Role`, `Snap`, `Diagonals`,
`PawnLabels`, `PawnKind`, `Size`, `HPBand` (`healthy`, `bruised`, `bloody`,
`veryBloody`, `nearDeath`, `dead`), `ConditionColor`, `ClearTrigger`,
`ShapeKind`, `FogMode`.

Sizes follow 5e footprints, not the old client's multipliers: tiny occupies one
cell and is drawn at half a cell; small and medium occupy one cell; large 2 by
2; huge 3 by 3; gargantuan 4 by 4. `func (s Size) Footprint() int` returns 1, 1,
1, 2, 3, 4.

Objects are props and vehicles: a wagon, a boat, a door. An object pawn has
`kind == object`, a rectangular footprint in `FootprintW` by `FootprintH`
cells, no size, optional hit points and armour class (nil is allowed), no
conditions, and draws as a rectangle rather than a disc.
`func (p Pawn) Footprint() (w, h int)` returns the object fields for an
object and the size's footprint on both axes for a creature. Every place
that needs a footprint calls it.

**Canonical ordering.** `Players`, `Pawns` and `Strokes` are kept sorted by id
ascending; `Fog` and `Initiative.Entries` keep insertion order because
order is meaning. `State.Normalize()` enforces this and every Apply calls it
before returning, so two states that are equal are byte-equal when marshalled.
That is what makes the golden tests and the convergence test simple.

**Defaults.** `NewState(roomID, name)` returns a state with `Schema`, one layer
named "Ground floor" with no map, fog disabled and prefill true, set as the
active layer, the grid defaults above, `PawnLabels` default, `PlayersCanDraw`
true, and everything else empty. `Table.Layers` is never empty and
`ActiveLayer` always names one of them.

**Layers.** A layer is a floor or a scene: a named slot with at most one map.
Only the active layer is shown to players, and every pawn, fog shape and stroke
belongs to exactly one layer. The grid is room-wide on the assumption that
layers share an origin and a scale; nothing enforces equal sizes, and a
mismatch is a visible misalignment rather than an error.

## Audiences and emissions

```go
type Audience int
const (
    ToAll      Audience = iota // every connected client
    ToGM                       // the owner's clients
    ToPlayers                  // every client whose role is player
    ToSender                   // the actor's clients only
    ToOthers                   // everyone but the actor
    ToPlayer                   // one player id, in Emission.Player
)

type Emission struct {
    Event  Event
    To     Audience
    Player ulid.ULID // ToPlayer only
}
```

Apply functions return `[]Emission`. They are the only place that decides who
sees what. The hub in phase 3 resolves an audience to a set of connections,
stamps the header, encodes once per distinct event, and fans out. It does no
projection of its own.

`State.Project(role Role) State` produces the snapshot for one audience. For
`RoleGM` it is a copy. For `RolePlayer`:

- pawns that are not **shown** are removed, where shown means `Visible` and
  `LayerID == Table.ActiveLayer`;
- initiative entries whose pawn is hidden are removed; entries for pawns on
  another layer stay, and the client renders them by name;
- each remaining pawn passes through `projectPawn`.

`projectPawn(p Pawn, t Table) Pawn` for the player audience: a `player` pawn is
unchanged, and so is an `object`. A `monster` or `npc` pawn keeps `HP`, `MaxHP`
and `AC` when `PawnLabels` is `full`; has all three set to nil and `HPBand`
computed on `default`; has all four nil on `none`. Bands, tested downward with
the first match winning: dead at 0, near death at or below a twentieth of max,
very bloody at or below a quarter, bloody at or below a half, bruised at or
below three quarters, healthy otherwise.

Every emission of a pawn to `ToPlayers` or `ToAll` goes through `projectPawn`
for the player copy. The rule for `ToAll` with a pawn payload is therefore two
encodings, one per audience, which the hub handles by asking the emission for
`Event.ForRole(role)`; events without pawn payloads return themselves.

## Commands

```go
type Actor struct {
    ID   ulid.ULID
    Role Role
}

type Env struct {
    NewID func() ulid.ULID // monotonic in production, deterministic in tests
}

type Command interface {
    Authorize(s *State, a Actor) error
    Apply(s *State, a Actor, env Env) ([]Emission, error)
}

type Error struct {
    Code    string // forbidden | invalid | not_found | locked
    Heading string
    Message string
}
```

`Authorize` never mutates. `Apply` assumes `Authorize` passed and may still
return an `Error` for `not_found` and `invalid` conditions that need the state
(a pawn id that is not there, a stroke that is already done). Every Apply ends
with `s.Normalize()`.

`DecodeCommand(b []byte) (Command, string, error)` reads `type` and `cid` from
the envelope, looks the type up in the wire registry, decodes into a fresh
value of that type with `DisallowUnknownFields`, and returns the command and
the correlation id. Unknown types and malformed JSON are `invalid`. Hub-only
commands are in a second registry that `DecodeCommand` never consults.

Validation limits, as named constants in `validate.go`:

| Constant | Value | Applies to |
| --- | --- | --- |
| `NameLimit` | 128 runes | pawn names, initiative names, room name |
| `ConditionNameLimit` | 64 runes | condition names |
| `CoordLimit` | ±1,000,000 | every x, y, offset |
| `HPLimit` | 0..9,999 | hp, maxHp; maxHp at least 1 |
| `ACLimit` | 0..99 | ac |
| `CellSizeMin`, `CellSizeMax` | 8, 512 | grid |
| `StrokeWidthMax` | 64 | stroke width |
| `StrokeChunkMax` | 512 numbers | one begin or extend |
| `StrokePointsMax` | 20,000 numbers | one stroke in total |
| `StrokesMax` | 5,000 | strokes per room |
| `FogPointsMax` | 2,000 numbers | one polygon |
| `FogShapesMax` | 2,000 | shapes per room |
| `PawnsMax` | 1,000 | pawns per room |
| `ConditionsMax` | 16 | conditions per pawn |
| `SelectionMax` | 200 | pawns in one move, drag or remove |
| `FootprintMax` | 20 | object footprint, per axis |
| `InitiativeMax` | 200 | entries |
| `LayersMax` | 20 | layers per room |

Colors validate as `#` plus 6 or 8 hex digits. Point arrays validate as even
length, at least the minimum for their kind, every value inside `CoordLimit`.

### Wire commands

**Shown** means visible and on the active layer, and it is the predicate every
player-facing pawn emission uses. Any command that changes a pawn's shown state
in either direction, whether by visibility, by layer, or by the active layer
moving, emits `pawn.removed` or `pawn.spawned` to players. A player's command
that names a layer other than the active one is `forbidden`.

"GM" means `a.Role == RoleGM`. "Owner" means the pawn's `OwnerID` equals
`a.ID`. "Own stroke" means the stroke's `By` equals `a.ID`. Commands carry
`cid` in the envelope; it is not a field of the Go type.

| Type | Fields | Authorize | Apply and emissions |
| --- | --- | --- | --- |
| `table.addLayer` | `name` | GM | `invalid` past `LayersMax`. Append a layer with a new id, no map, fog defaults. Emit `table.updated` ToAll. |
| `table.removeLayer` | `layer` | GM | `invalid` for the last layer. Remove every pawn on it (their initiative entries too), every fog shape and stroke on it, and the layer. If it was active, the layer below becomes active, or the one above for the bottom layer. Emit, in order: `pawn.removed` per pawn ToGM (and ToPlayers if shown), `fog.cleared {layer}` and `stroke.cleared {layer}` ToAll, `initiative.updated` ToAll when entries changed, `table.updated` ToAll, then `pawn.spawned` ToPlayers for each pawn newly shown on the new active layer. |
| `table.renameLayer` | `layer`, `name` | GM | Emit `table.updated` ToAll. |
| `table.moveLayer` | `layer`, `index` | GM | Reorder. Emit `table.updated` ToAll. |
| `table.setLayerMap` | wire: `layer`, `assetId`. The hub resolves it before Apply into `Map MapRef` tagged `json:"-"` | GM | `invalid` if unresolved. Set that layer's map. Emit `table.updated` ToAll. |
| `table.clearLayerMap` | `layer` | GM | Set that layer's map to nil. Emit `table.updated` ToAll. |
| `table.setActiveLayer` | `layer` | GM | Emit `table.updated` ToAll, then ToPlayers `pawn.removed` for every pawn shown before and not after, and `pawn.spawned` for every pawn shown after and not before. |
| `table.setGrid` | `grid Grid` | GM | Validate every field, set, emit `table.updated` ToAll. |
| `table.setOptions` | `pawnLabels`, `playersCanDraw` | GM | Set both. Emit `table.updated` ToAll. When `pawnLabels` changed, also emit `pawn.updated` ToPlayers for every visible monster and npc pawn, since their projection changed. |
| `pawn.spawn` | wire: `kind`, `layer`, `x`, `y`, `visible`, one of `monsterId`, `characterId`, `assetId` (a token), and optional `name` for an npc or object, and `footprintW`, `footprintH` for an object. The hub resolves it before Apply into `Pawn Pawn` tagged `json:"-"`, without an id | GM; or a player when `kind == player` and `characterId` is the player's own session character | `invalid` if unresolved. Assign id, snap position per grid, `Z` = one above the current max. Emit `pawn.spawned` ToGM; ToPlayers only if shown. |
| `pawn.spawnCharacters` | none on the wire. The hub resolves it into `Pawns []Pawn` tagged `json:"-"`: one pawn per connected player who joined with a character and has no player pawn yet, placed in a row at the map centre | GM | For each resolved pawn, exactly as `pawn.spawn`. |
| `pawn.move` | `anchor`, `x`, `y`, `others []ulid` | GM, or owner of the anchor and of every id in `others` | All-or-nothing. Snap the anchor per its footprint, compute one delta from the anchor's committed position, apply the delta to every id in `others`, so relative positions are preserved exactly. Emit `pawn.moved` with the list of resulting positions: ToGM complete, ToPlayers with hidden pawns dropped, suppressed if that leaves none. |
| `pawn.drag` | `anchor`, `x`, `y`, `others []ulid` | as `pawn.move` | No state change, no snapping. Compute the delta from the anchor's committed position and emit `pawn.dragging` with every pawn's previewed position ToOthers; the player copy drops hidden pawns and is suppressed if none remain. |
| `pawn.update` | `id`, optional `name`, `hp`, `maxHp`, `ac`, `size`, `z`, and for an object `footprintW`, `footprintH` | GM or owner | Apply present fields, validate, clamp `hp` to `0..maxHp`. Emit `pawn.updated` ToGM; ToPlayers only if shown. |
| `pawn.setConditions` | `id`, `conditions []Condition` (ids assigned for new ones) | GM or owner | `invalid` for an object. Replace the list. Emit `pawn.updated` as above. |
| `pawn.setVisible` | `id`, `visible` | GM | Set. Emit `pawn.updated` ToGM. If it stopped being shown, emit `pawn.removed` ToPlayers and, when an initiative entry references it, `initiative.updated` ToPlayers. If it became shown, emit `pawn.spawned` ToPlayers and, likewise, `initiative.updated` ToPlayers. |
| `pawn.setLayer` | `ids []ulid`, `layer` | GM | Set every pawn's `LayerID`. Emit `pawn.updated` per pawn ToGM, and ToPlayers `pawn.removed` or `pawn.spawned` for each pawn whose shown state flipped. Players never move pawns between layers because they would then hold a pawn they cannot see. |
| `pawn.remove` | `ids []ulid` | GM | For each id: remove the pawn and any initiative entries that reference it; if the active entry was removed, active moves to the next entry or nil. Emit one `pawn.removed` per pawn ToGM and ToPlayers if it was shown, then one `initiative.updated` ToAll when entries changed. |
| `initiative.set` | `entries []InitiativeEntry` (ids assigned for new ones), `active *ulid` | GM | Replace, validate every `pawnId` exists. Emit `initiative.updated` ToAll. |
| `initiative.next` | none | GM, or the player who owns the active entry's pawn | Advance to the next entry, wrapping and incrementing `Round`; from nil, activate the first and set `Round` 1. Apply condition durations: on the pawn whose turn ends, decrement conditions with `Clear == end`; on the pawn whose turn starts, decrement those with `Clear == start`; remove any that reach zero; a duration of -1 never changes. Emit `initiative.updated` ToAll plus `pawn.updated` for each pawn whose conditions changed, with the usual visibility rule. |
| `initiative.clear` | none | GM | Empty entries, nil active, zero round. Emit `initiative.updated` ToAll. |
| `fog.setEnabled` | `layer`, `enabled` | GM | Set the layer's flag. Emit `table.updated` ToAll. |
| `fog.setPrefill` | `layer`, `prefill` | GM | Set the layer's flag. Emit `table.updated` ToAll. |
| `fog.add` | `layer`, `kind`, `mode`, `points` | GM | Assign id, append. Emit `fog.added` ToAll. |
| `fog.remove` | `id` | GM | Emit `fog.removed` ToAll. |
| `fog.clear` | `layer` | GM | Remove that layer's shapes. Emit `fog.cleared {layer}` ToAll. |
| `stroke.begin` | `id` (client ULID), `layer`, `color`, `width`, `points` | GM, or a player when `PlayersCanDraw` | Id must parse and be unused. Append a stroke with `By = a.ID`, `Done = false`. Emit `stroke.began` ToAll. |
| `stroke.extend` | `id`, `points` | own stroke, not done | Append points within limits. Emit `stroke.extended` ToAll. |
| `stroke.end` | `id` | own stroke | `Done = true`. Emit `stroke.ended` ToAll. |
| `stroke.erase` | `ids []ulid` | GM, or every id is an own stroke | Remove. Emit `stroke.erased` ToAll. |
| `stroke.clear` | `layer` | GM | Remove that layer's strokes. Emit `stroke.cleared {layer}` ToAll. |
| `ping` | `layer`, `x`, `y` | anyone | No state change. Emit `pinged` ToAll with the actor and the layer; clients show it only when viewing that layer. |
| `player.kick` | `id` | GM, target is not the GM | Remove the player and, for their pawns of kind player, nothing; pawns stay. Emit `player.kicked` ToPlayer(id) and `player.left` ToAll. The hub closes the target's connections and clears their session membership on seeing `player.kicked`. |
| `sync.request` | none | anyone | No state change. Emit `snapshot` ToSender, projected for the actor's role. |

Commands that carry a `json:"-"` field are **resolved commands**: the wire
shape is what the client sends and what the generator emits, and the resolved
field is filled by the hub from the database before `Apply` runs. `Apply` on an
unresolved command returns `invalid`, so a test that forgets to resolve fails
loudly rather than spawning an empty pawn.

### Hub-only commands

Never decodable from the wire. The hub builds them from connection events and
HTTP handlers.

| Type | Fields | Apply and emissions |
| --- | --- | --- |
| `player.join` | `player Player` | Insert or replace by id with `Connected = true`. Emit `player.joined` ToAll if new, `player.updated` ToAll if returning. |
| `player.setConnected` | `id`, `connected` | Set. Emit `player.updated` ToAll. The player row stays so a reconnect resumes them. |
| `player.leave` | `id` | Remove the player. Emit `player.left` ToAll. Used when a player leaves via the HTTP route. |
| `room.setLocked` | `locked` | Set `Room.Locked`. Emit `room.updated` ToAll. The HTTP lock route is the writer of record; the hub mirrors. |
| `room.setName` | `name` | Emit `room.updated` ToAll. |
| `room.close` | none | Emit `room.closed` ToAll. The hub then drops every connection and unloads the room. |

## Events

```go
type Header struct {
    Type string     `json:"type"`
    Seq  uint64     `json:"seq"`
    By   *ulid.ULID `json:"by,omitempty"`
}

type Event interface {
    eventType() string
    header() *Header
}
```

Every event struct embeds `Header` as its first field, so `encoding/json`
flattens it. `EncodeEvent(ev Event, seq uint64, by *ulid.ULID) ([]byte, error)`
sets the header and marshals. Events with a pawn payload also implement
`ForRole(Role) Event`, returning a copy with the pawn projected; everything else
returns itself.

Events and their payloads, by sizing rule:

| Type | Payload | Rule |
| --- | --- | --- |
| `snapshot` | `state State` (already projected), `you` `{id, role}`, `version string` | singleton |
| `room.updated` | `room RoomInfo` | singleton |
| `room.closed` | none | transient |
| `table.updated` | `table Table` | singleton |
| `player.joined`, `player.updated` | `player Player` | collection item |
| `player.left` | `id` | collection item |
| `player.kicked` | `reason string` | transient |
| `pawn.spawned`, `pawn.updated` | `pawn Pawn` | collection item |
| `pawn.removed` | `id` | collection item |
| `pawn.moved` | `pawns []PawnPosition`, each `{id, x, y}` | hot path |
| `pawn.dragging` | `pawns []PawnPosition` | transient hot path |
| `initiative.updated` | `initiative Initiative` | singleton |
| `fog.added` | `shape FogShape` | collection item |
| `fog.removed` | `id` | collection item |
| `fog.cleared` | `layer` | collection |
| `stroke.began` | `stroke Stroke` | collection item |
| `stroke.extended` | `id`, `points []int` | hot path |
| `stroke.ended` | `id` | collection item |
| `stroke.erased` | `ids []ulid` | collection |
| `stroke.cleared` | `layer` | collection |
| `pinged` | `layer`, `x`, `y` | transient |
| `error` | `cid`, `code`, `heading`, `message` | transient, ToSender, built by the hub from `Error` |

`Transient() bool` on each event type marks the ones the reducer ignores:
`room.closed`, `player.kicked`, `pawn.dragging`, `pinged`, `error`. The
generator emits that list as a TypeScript constant so the client's reducer and
effects layer split on the same set.

## Snapping

`snap.go`:

```go
func SnapAxis(cell, offset, footprint int, mode Snap, v int) int
func SnapPoint(g Grid, footprintW, footprintH int, x, y int) (int, int)
```

Snapping is decided per axis by that axis's footprint parity, so a two by four
wagon snaps both axes to vertices and a three by four object snaps x to a cell
centre and y to a vertex.

- `Snap == off`: unchanged.
- Odd footprint under `cells`, or even footprint under `corners`: snap to the
  nearest cell centre: `round((v - offset - cell/2) / cell) * cell + offset + cell/2`.
- Even footprint under `cells`, or odd footprint under `corners`: snap to the
  nearest vertex: `round((v - offset) / cell) * cell + offset`.

Compute in float64, round half away from zero, return ints. `pawn.spawn` and
`pawn.move` snap the anchor only. The others in a group move by the anchor's
delta, unsnapped, so riders stay exactly where they sat on the wagon. `pawn.drag`
never snaps; the dragging client snaps its own preview and the server's delta
is computed from the raw point, so a snapping client and a non-snapping one see
the same ghosts.

## Snapshot

`snapshot.go`:

```go
const Schema = 1
func Marshal(s *State) ([]byte, error)          // the GM-complete state
func Unmarshal(b []byte) (*State, error)        // rejects Schema != Schema with ErrSchema
```

`Unmarshal` of an empty object (the column default) returns `ErrEmpty`; the hub
treats both errors as "start fresh" and logs the schema case. `Seq` is stored
in the snapshot and mirrored to `rooms.snapshot_seq` by the hub. Schema
migrations are out of scope until a second schema exists.

## Reference reducer

`reduce.go` exports `Reduce(s *State, ev Event) error`. It applies one event to
a projected state exactly as the TypeScript reducer will: singletons replace,
collection items upsert or delete by id, hot paths mutate the named fields,
transient events are ignored. It is written for clarity, not speed, and is
used by the tests below and by the fixture generator. It is also the
specification the phase 3 TypeScript reducer is ported from.

## TypeScript generation

`gen/main.go`, invoked by a `//go:generate go run ./gen` directive in
`state.go`, writes `server/js/room/protocol.ts`. It reflects over the wire
command registry, the event registry, `State`, and every type reachable from
them. Rules:

- Struct to `export interface`, field names from json tags, embedded structs
  flattened.
- `ulid.ULID` and `*ulid.ULID` to `string` and `string | null`.
- Other pointers to `T | null`; `omitempty` adds `?`.
- `int`, `uint64`, `float64` to `number`; `bool`, `string` as themselves.
- Slices to `T[]`, maps to `Record<string, T>`.
- Enum types to a literal union built from `Values()`.
- Each command interface gets `type: "pawn.move"` and `cid: string`; each event
  gets `type: "pawn.moved"`, `seq: number`, `by?: string`.
- `export type Command = ...` and `export type Event = ...` unions, and
  `export const TRANSIENT_EVENTS: ReadonlySet<Event["type"]>`.
- A header comment saying the file is generated and how.

tygo was considered and rejected: it emits interfaces but cannot emit the
discriminated unions from the registry, and the registry is the one place the
type strings already live.

`TestProtocolTypesAreCurrent` regenerates into a buffer and compares with the
committed file, so a stale file fails `make check`.

## Tests

Table-driven and pure. Fixtures build states with a deterministic `Env.NewID`
that returns ULIDs in sequence.

- **Authorization**: every wire command against GM, owner, other player. One
  table, one assertion per cell.
- **Validation**: each limit at the boundary and one past it.
- **Snapping**: the four parity and mode combinations, negative offsets, a
  point exactly between two centres, and a two by three object snapping
  differently on each axis.
- **Group move**: a wagon with three riders moves as one delta with riders'
  offsets preserved to the pixel; a player anchoring on their own pawn with a
  monster in `others` is `forbidden` and nothing moves; `SelectionMax` plus
  one is `invalid`; the player copy of `pawn.moved` omits a hidden rider and is
  absent when every pawn in the list is hidden.
- **Objects**: spawn with a footprint of 0 or 21 is `invalid`; `pawn.setConditions`
  on an object is `invalid`; `hp` may be nil.
- **Visibility transitions**: `pawn.setVisible` each way emits exactly the
  events in the table to exactly the audiences, including the initiative
  side-effect.
- **Projection**: `PawnLabels` in all three settings against player, monster and
  npc pawns, hit points and armour class both; `table.setOptions` changing it
  emits the pawn updates. `Normalize` repairs a setting nothing accepts, which
  is what every room saved as `monsterHp` reads back as.
- **Initiative**: next from nil, wrap and round increment, removal of the
  active entry, condition decrement on start and end triggers, -1 untouched.
- **Kick**: target gets `player.kicked`, everyone gets `player.left`, GM cannot
  be kicked.
- **Layers**: setting the active layer emits exactly the removed and spawned
  sets for players and nothing for the GM beyond `table.updated`; removing a
  layer with pawns, fog and strokes emits the documented sequence in order and
  moves the active layer when needed; removing the last layer is `invalid`;
  a player's `ping`, `stroke.begin`, `fog.add` or `pawn.spawn` on a non-active
  layer is `forbidden`; `pawn.setLayer` by a player is `forbidden`; the
  initiative projection keeps an entry whose pawn is on another layer.
- **Decode**: unknown type, unknown field, hub-only type from the wire, bad
  ULID, all `invalid`.
- **Encode**: header fields present and first, `by` omitted when nil.
- **Snapshot**: round trip; empty object is `ErrEmpty`; wrong schema is
  `ErrSchema`; marshalled output is byte-stable across two marshals of equal
  states.
- **Convergence**: a scripted scenario of about fifty commands covering every
  family. After each command, for each role: reduce the emitted events for
  that role onto the projected before-state and compare with the projected
  after-state by marshalled bytes. This is the test the overview promises.
- **Golden fixtures**: the same scenario writes
  `testdata/reducer/<role>.json` as `{ "initial": State, "steps": [{ "events": [...], "state": State }] }`.
  Regenerate with `go test ./internal/room -update`; the test without the flag
  compares. Phase 3's TypeScript test replays these.

## Verification

`make check` is the whole verification. There is nothing to click.

## Out of scope

The hub, sockets, sequence assignment across connections, rate limiting,
resolution of set-map and spawn against the database, the TypeScript reducer,
and any schema migration of snapshots.
