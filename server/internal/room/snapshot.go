package room

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oklog/ulid/v2"
)

// Schema is the version of the shape in state.go. It is stored inside the
// snapshot rather than in a column beside it, so a snapshot that gets copied,
// dumped or pasted somewhere carries its own version with it.
//
// THERE IS NO MIGRATION PATH AND THAT IS DELIBERATE FOR NOW. A snapshot from a
// different schema starts the room fresh, which costs a GM the pawns they had
// placed and costs nobody a session, because the only time the number changes
// is a deploy that changed the shape. When a second schema exists, the answer
// is a migration function per step and this constant is what selects it -- not
// a best-effort decode, which is how a room comes back half-populated.
// SCHEMA 2 MOVED AN OBJECT'S SIZE FROM CELLS TO MAP PIXELS. footprintW and
// footprintH became width and height, and a 2 that meant two cells would decode
// as two pixels -- an invisible wagon rather than a decode error, which is
// exactly the kind of failure a version number exists to turn into a loud one.
const Schema = 2

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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return nil, fmt.Errorf("room: unreadable snapshot: %w", err)
	}
	if len(fields) == 0 {
		return nil, ErrEmpty
	}

	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("room: unreadable snapshot: %w", err)
	}
	if s.Schema != Schema {
		return nil, fmt.Errorf("%w: found %d, want %d", ErrSchema, s.Schema, Schema)
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

	c.Initiative.Active = cloneID(s.Initiative.Active)
	c.Initiative.Entries = make([]InitiativeEntry, len(s.Initiative.Entries))
	for i, e := range s.Initiative.Entries {
		e.PawnID = cloneID(e.PawnID)
		c.Initiative.Entries[i] = e
	}

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

func hasEntry(entries []InitiativeEntry, id ulid.ULID) bool {
	for _, e := range entries {
		if e.ID == id {
			return true
		}
	}

	return false
}

// projectPawn is the hit-point half of the projection, and it is where the
// room's one setting is read.
//
// PLAYER PAWNS AND OBJECTS PASS THROUGH UNCHANGED. A player's own character
// sheet is not a secret from the table, and a door's hit points are the thing
// the party is currently hitting. The setting is about monsters, which is what
// it is called.
func projectPawn(p Pawn, t Table) Pawn {
	if p.Kind != PawnMonster && p.Kind != PawnNPC {
		return p
	}

	switch t.MonsterHP {
	case HPExact:
		return p

	case HPBandOn:
		band := hpBand(p.HP, p.MaxHP)
		p.HP = nil
		p.MaxHP = nil
		p.HPBand = band

		return p

	default:
		p.HP = nil
		p.MaxHP = nil
		p.HPBand = nil

		return p
	}
}

// hpBand is the coarse health a player is told about instead of a number.
//
// The thresholds are 5e's own vocabulary: bloodied is at or below half, which
// is the word every table already uses, and critical is at or below a quarter,
// which is where a party decides whether to spend the last healing spell. The
// comparisons multiply rather than divide so that a maximum of 7 has exact
// thresholds rather than ones that depend on which way integer division fell.
func hpBand(hp, maxHP *int) *HPBand {
	if hp == nil || maxHP == nil || *maxHP < 1 {
		return nil
	}

	band := BandHealthy
	switch {
	case *hp <= 0:
		band = BandDead
	case *hp*4 <= *maxHP:
		band = BandCritical
	case *hp*2 <= *maxHP:
		band = BandBloodied
	}

	return &band
}
