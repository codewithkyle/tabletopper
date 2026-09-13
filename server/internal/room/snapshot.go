package room

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/oklog/ulid/v2"
)

const Schema = 3

type fields map[string]json.RawMessage

var migrations = map[int]func(fields) error{
	1: migrateFootprints,
	2: migrateStrokeKinds,
}

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
func migrateStrokeKinds(f fields) error {
	raw, ok := f["strokes"]
	if !ok {
		return nil
	}
	var strokes []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &strokes); err != nil {
		return fmt.Errorf("strokes: %w", err)
	}
	for _, st := range strokes {
		st["kind"] = json.RawMessage(`"` + string(StrokeFree) + `"`)
	}
	out, err := json.Marshal(strokes)
	if err != nil {
		return fmt.Errorf("strokes: %w", err)
	}
	f["strokes"] = out
	return nil
}
func number(n int) json.RawMessage {
	return json.RawMessage(strconv.Itoa(n))
}
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
	ErrEmpty  = errors.New("room: the snapshot is empty")
	ErrSchema = errors.New("room: the snapshot is from a different schema")
)

func Marshal(s *State) ([]byte, error) {
	s.Normalize()
	return json.Marshal(s)
}
func Unmarshal(b []byte) (*State, error) {
	if len(b) == 0 {
		return nil, ErrEmpty
	}
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
func (s *State) Clone() State {
	c := *s
	c.Table.Layers = cloneLayers(s.Table.Layers)
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
	c.Rolls = make([]Roll, len(s.Rolls))
	for i, r := range s.Rolls {
		c.Rolls[i] = cloneRoll(r)
	}
	c.Music = CloneMusic(s.Music)
	c.Normalize()
	return c
}
func ClonePawn(p Pawn) Pawn { return clonePawn(p) }
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
func clonePoint(v *Point) *Point {
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
func hasEntry(entries []InitiativeEntry, id ulid.ULID) bool {
	for _, e := range entries {
		if e.ID == id {
			return true
		}
	}
	return false
}
