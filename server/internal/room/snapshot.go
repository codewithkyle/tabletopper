package room

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/oklog/ulid/v2"
)

// Schema is the version of the shape in state.go. It is stored inside the
// snapshot rather than in a column beside it, so a snapshot that gets copied,
// dumped or pasted somewhere carries its own version with it.
//
// A SNAPSHOT FROM AN OLDER SCHEMA IS MIGRATED, ONE STEP AT A TIME. Unmarshal
// reads the number, runs migrations[n] for every n from the one found up to
// this one, and only then decodes into State -- so a room written before a
// shape change comes back with its table rather than empty. A snapshot from a
// NEWER schema is ErrSchema and starts the room fresh: that is a rollback, and
// the hub keeps the bytes it could not read so nothing is lost when the
// forward build returns.
//
// BUMPING THIS NUMBER MEANS WRITING A MIGRATION, and the test over the golden
// snapshots in testdata is what says so: every past schema has one there, and
// each has to decode to today's shape.
//
// SCHEMA 2 MOVED AN OBJECT'S SIZE FROM CELLS TO MAP PIXELS. footprintW and
// footprintH became width and height, and a 2 that meant two cells would decode
// as two pixels -- an invisible wagon rather than a decode error, which is
// exactly the kind of failure a version number exists to turn into a loud one.
const Schema = 2

// fields is a snapshot half-decoded: the top-level object as raw JSON per key,
// which is the shape a migration edits. Nothing below the keys a step touches
// is decoded, so a step written for schema 1 goes on working however the
// shape changes after it.
type fields map[string]json.RawMessage

// migrations is one step per past schema, keyed by the schema it reads and
// producing the next. A step edits the raw fields in place and may fail, which
// Unmarshal reports as an unreadable snapshot.
var migrations = map[int]func(fields) error{
	1: migrateFootprints,
}

// migrateFootprints is schema 1 to 2: an object's footprintW and footprintH,
// in cells, become width and height in map pixels at the room's own cell
// size, and every pawn gains a rotation of zero. A creature's footprint was
// derived from its size category and is dropped -- see Pawn, which says why a
// creature has a Size and an object has a Width.
func migrateFootprints(f fields) error {
	var table struct {
		Grid struct {
			CellSize int `json:"cellSize"`
		} `json:"grid"`
	}
	if raw, ok := f["table"]; ok {
		if err := json.Unmarshal(raw, &table); err != nil {
			return fmt.Errorf("table: %w", err)
		}
	}
	cell := max(1, table.Grid.CellSize)

	raw, ok := f["pawns"]
	if !ok {
		return nil
	}

	var pawns []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pawns); err != nil {
		return fmt.Errorf("pawns: %w", err)
	}

	for _, p := range pawns {
		var kind string
		var w, h int
		_ = json.Unmarshal(p["kind"], &kind)
		_ = json.Unmarshal(p["footprintW"], &w)
		_ = json.Unmarshal(p["footprintH"], &h)
		delete(p, "footprintW")
		delete(p, "footprintH")

		width, height := 0, 0
		if kind == string(PawnObject) {
			width, height = max(1, w)*cell, max(1, h)*cell
		}

		p["width"] = number(width)
		p["height"] = number(height)
		p["rotation"] = number(0)
	}

	out, err := json.Marshal(pawns)
	if err != nil {
		return fmt.Errorf("pawns: %w", err)
	}
	f["pawns"] = out

	return nil
}

func number(n int) json.RawMessage {
	return json.RawMessage(strconv.Itoa(n))
}

// migrate brings half-decoded fields from the schema they carry up to this
// build's, and answers ErrSchema for one this build has never seen.
func migrate(f fields) error {
	found := 0
	if raw, ok := f["schema"]; ok {
		if err := json.Unmarshal(raw, &found); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}

	if found > Schema {
		return fmt.Errorf("%w: found %d, want %d", ErrSchema, found, Schema)
	}

	for n := found; n < Schema; n++ {
		step, ok := migrations[n]
		if !ok {
			return fmt.Errorf("%w: found %d and there is no migration from it", ErrSchema, found)
		}
		if err := step(f); err != nil {
			return fmt.Errorf("room: migrating a snapshot from schema %d: %w", n, err)
		}
	}

	f["schema"] = number(Schema)

	return nil
}

var (
	// ErrEmpty is the column default. A room row is created with an empty JSON
	// object and stays that way until the first snapshot, so this is what the
	// hub sees on the first join to a room nobody has opened yet. It is not a
	// problem and is not logged.
	ErrEmpty = errors.New("room: the snapshot is empty")

	// ErrSchema is a snapshot this build cannot read. The hub starts the room
	// fresh, like ErrEmpty, but logs it -- one is the ordinary first join and
	// the other means a live table just lost its pawns to a deploy.
	ErrSchema = errors.New("room: the snapshot is from a different schema")
)

// Marshal encodes the GM-complete state: everything, projected for nobody. It
// is what goes in the snapshot column, and it normalizes first so that two
// rooms holding the same facts produce the same bytes -- which is what makes a
// golden fixture a fixture rather than a rendering.
func Marshal(s *State) ([]byte, error) {
	s.Normalize()

	return json.Marshal(s)
}

// Unmarshal decodes a snapshot column. Both of its errors mean "start fresh" to
// the caller; they are distinct so that the caller can tell an ordinary first
// join from a schema break worth writing to the log.
func Unmarshal(b []byte) (*State, error) {
	if len(b) == 0 {
		return nil, ErrEmpty
	}

	// The emptiness test is on the decoded object rather than on the bytes,
	// because the column default is written by MySQL as json_object() and
	// comes back with whatever spacing the server chose.
	var f fields
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("room: unreadable snapshot: %w", err)
	}
	if len(f) == 0 {
		return nil, ErrEmpty
	}

	if err := migrate(f); err != nil {
		return nil, err
	}

	migrated, err := json.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("room: unreadable snapshot: %w", err)
	}

	var s State
	if err := json.Unmarshal(migrated, &s); err != nil {
		return nil, fmt.Errorf("room: unreadable snapshot: %w", err)
	}

	s.Normalize()

	return &s, nil
}

// Clone is a deep copy. Project returns one, and every caller of Project is
// handed something it may hold on to and mutate: the snapshot event goes into
// an encoder, a test compares it against a reduced state. Sharing a backing
// array with the live room would make either of those a way to corrupt it.
//
// The pointers are copied too, not just the slices. Nothing in this package
// writes through a *int today, but "nothing writes through it" is a property
// that stops being true the first time somebody adds a field.
func (s *State) Clone() State {
	c := *s

	c.Table.Layers = make([]Layer, len(s.Table.Layers))
	for i, l := range s.Table.Layers {
		l.Map = cloneRef(l.Map)
		c.Table.Layers[i] = l
	}

	c.Players = make([]Player, len(s.Players))
	for i, p := range s.Players {
		p.CharacterID = cloneID(p.CharacterID)
		c.Players[i] = p
	}

	c.Pawns = make([]Pawn, len(s.Pawns))
	for i, p := range s.Pawns {
		c.Pawns[i] = clonePawn(p)
	}

	c.Initiative = cloneInitiative(s.Initiative)

	c.Fog = make([]FogShape, len(s.Fog))
	for i, f := range s.Fog {
		c.Fog[i] = cloneShape(f)
	}

	c.Strokes = make([]Stroke, len(s.Strokes))
	for i, st := range s.Strokes {
		c.Strokes[i] = cloneStroke(st)
	}

	c.Normalize()

	return c
}

func clonePawn(p Pawn) Pawn {
	p.HP = cloneInt(p.HP)
	p.MaxHP = cloneInt(p.MaxHP)
	p.AC = cloneInt(p.AC)
	p.HPBand = cloneBand(p.HPBand)
	p.OwnerID = cloneID(p.OwnerID)
	p.MonsterID = cloneID(p.MonsterID)
	p.CharacterID = cloneID(p.CharacterID)
	p.Conditions = cloneSlice(p.Conditions)

	return p
}

func cloneInt(v *int) *int {
	if v == nil {
		return nil
	}
	c := *v

	return &c
}

func cloneID(v *ulid.ULID) *ulid.ULID {
	if v == nil {
		return nil
	}
	c := *v

	return &c
}

func cloneBand(v *HPBand) *HPBand {
	if v == nil {
		return nil
	}
	c := *v

	return &c
}

func cloneRef(v *MapRef) *MapRef {
	if v == nil {
		return nil
	}
	c := *v

	return &c
}

// Project is the whole of the two-audience rule, in one place.
//
// A HIDDEN PAWN MUST NOT REACH A PLAYER'S BROWSER AT ALL. The old app sent it
// with a hidden flag set and trusted the client to respect it, which is a cheat
// that costs one devtools tab. Removing it here means the information is not on
// the wire, and there is nothing to respect.
//
// WHAT IS NOT PROJECTED, deliberately, and recorded so it is a decision rather
// than an oversight: the layer list. Players receive every layer's name and
// map reference, and every layer's fog and strokes, because table.updated,
// fog.added and stroke.began are all sent to everybody and a snapshot that
// disagreed with the events would be a second, subtly different truth. Closing
// that gap means projecting those events too, which is the same shape of work
// as the fog-aware pawn projection the overview already defers.
func (s *State) Project(role Role) State {
	c := s.Clone()
	if role == RoleGM {
		return c
	}

	pawns := make([]Pawn, 0, len(c.Pawns))
	for _, p := range c.Pawns {
		if !s.Shown(p) {
			continue
		}
		pawns = append(pawns, projectPawn(p, c.Table))
	}
	c.Pawns = pawns

	// The tracker filters by the same rule the events use, which is
	// projectInitiative and is written out beside the commands that emit it.
	c.Initiative = projectInitiative(s)

	c.Normalize()

	return c
}

// Health is the band an interface draws a creature's injuries from, and it is
// the Go twin of healthOf in server/js/room/render/wounds.ts.
//
// THE NUMBER WINS WHERE BOTH ARRIVE, which is every player character in every
// room and every monster in a room whose labels are full. The band is what a
// viewer was given INSTEAD of the numbers, so it is only read when there is no
// number to read -- and nil is the third answer, for a creature this viewer was
// told nothing about at all.
//
// IT IS NOT hpBand AND IT IS NOT A SECOND COPY OF IT EITHER. hpBand turns a
// pair of numbers into a band; this decides WHICH of a projected pawn's two
// answers to ask, and then asks hpBand. The pair is the same one bandOf and
// healthOf make on the client, which is what stops a card and the sprite beside
// it from disagreeing.
func Health(p Pawn) *HPBand {
	if p.HP != nil {
		return hpBand(p.HP, p.MaxHP)
	}

	return p.HPBand
}

// Dead is a creature this viewer can see is finished.
//
// IT READS THE BAND RATHER THAN THE NUMBER, so that it answers the same for a
// GM holding "0 / 7" and for a player who was handed the word. A pawn nobody
// told this viewer anything about is not dead as far as they are concerned,
// which is the honest reading and is also what keeps the skip rule from
// silently passing over a monster in a room with its labels off -- the skip is
// decided on the GM's copy, where the numbers always are.
func Dead(p Pawn) bool {
	b := Health(p)

	return b != nil && *b == BandDead
}

func hasEntry(entries []InitiativeEntry, id ulid.ULID) bool {
	for _, e := range entries {
		if e.ID == id {
			return true
		}
	}

	return false
}

// projectPawn is the statistics half of the projection, and it is where the
// room's one label setting is read.
//
// PLAYER PAWNS AND OBJECTS PASS THROUGH UNCHANGED. A player's own character
// sheet is not a secret from the table, and a door's hit points are the thing
// the party is currently hitting. What the setting treats differently is a
// monster's.
//
// HIT POINTS ARE SENT TO EVERYONE AND HIDDEN BY THE INTERFACE, which is the
// opposite of what this function used to do and is a deliberate reversal. The
// line is now: WHAT THE TABLE HAS TO DRAW IS SENT, AND WHAT ONLY A PERSON WOULD
// READ IS PROJECTED. A creature's hit points are drawn -- the blood on it, the
// blood under it, the pallor and the heartbeat all read the number, and how
// hard a hit landed can only be the difference between two of them -- so the
// number has to be in the browser for the table to look right. Armour class is
// drawn by nothing, so it is still withheld here.
//
// THAT MEANS A PLAYER CAN READ A MONSTER'S HIT POINTS OUT OF THE SOCKET, and
// this app has decided not to fight that. A player who wants an edge already
// has one -- they can look the monster up -- and the answer to it is to change
// the monster rather than to change the app. What the setting still does is
// decide what the interface SHOWS, which is the honest description of a
// preference about how a table plays rather than a secret it cannot keep.
//
// THE BAND IS STILL SENT AND IS STILL THE INSTRUCTION. Its presence is how a
// viewer is told they get the word rather than the number, and it is the one
// piece of this the client does not have to work out for itself.
func projectPawn(p Pawn, t Table) Pawn {
	if p.Kind != PawnMonster && p.Kind != PawnNPC {
		return p
	}

	if t.PawnLabels == LabelsFull {
		return p
	}

	p.AC = nil
	p.HPBand = nil

	if t.PawnLabels == LabelsDefault {
		p.HPBand = hpBand(p.HP, p.MaxHP)
	}

	return p
}

// hpBand is the coarse health a player is told about instead of a number.
//
// THE CUTS ARE THREE QUARTERS, A HALF, A QUARTER AND A TWENTIETH, and each one
// is a boundary rather than a range: a creature is bruised from just under
// three quarters down to half, and bloody from there down to a quarter. The top
// band is wide because the answer above three quarters is always "it is fine",
// and the bottom two are narrow because that is the only part of the scale
// anybody is making a decision on.
//
// IT IS TESTED DOWNWARD AND THE FIRST MATCH WINS, which is what makes the
// overlapping way of saying it -- under a half is bloody, under a quarter is
// very bloody -- come out as one band per creature.
//
// THE COMPARISONS MULTIPLY RATHER THAN DIVIDE so that a maximum of 7 has exact
// thresholds rather than ones that depend on which way integer division fell.
func hpBand(hp, maxHP *int) *HPBand {
	if hp == nil || maxHP == nil || *maxHP < 1 {
		return nil
	}

	band := BandHealthy
	switch {
	case *hp <= 0:
		band = BandDead
	case *hp*20 <= *maxHP:
		band = BandNearDeath
	case *hp*4 <= *maxHP:
		band = BandVeryBloody
	case *hp*2 <= *maxHP:
		band = BandBloody
	case *hp*4 <= *maxHP*3:
		band = BandBruised
	}

	return &band
}
