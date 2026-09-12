package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

type FogAdded struct {
	Header
	Shape FogShape `json:"shape"`
}

func (*FogAdded) eventType() string { return "fog.added" }

type FogRemoved struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*FogRemoved) eventType() string { return "fog.removed" }

type FogSetEnabled struct {
	Layer   ulid.ULID `json:"layer"`
	Enabled bool      `json:"enabled"`
}

func (c *FogSetEnabled) Authorize(s *State, a Actor) error {
	return requireGM(a, "turn fog on or off")
}
func (c *FogSetEnabled) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	l.FogEnabled = c.Enabled
	s.Normalize()
	return nil, nil
}

type FogSetPrefill struct {
	Layer   ulid.ULID `json:"layer"`
	Prefill bool      `json:"prefill"`
}

func (c *FogSetPrefill) Authorize(s *State, a Actor) error {
	return requireGM(a, "change how a layer's fog works")
}
func (c *FogSetPrefill) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	l.FogPrefill = c.Prefill
	s.Normalize()
	return nil, nil
}

type FogAdd struct {
	Layer  ulid.ULID `json:"layer"`
	Kind   ShapeKind `json:"kind"`
	Mode   FogMode   `json:"mode"`
	Points []int     `json:"points"`
}

func (c *FogAdd) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the fog")
}
func (c *FogAdd) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}
	if !c.Kind.Valid() {
		return nil, invalid("Bad shape", "That is not a fog shape.")
	}
	if !c.Mode.Valid() {
		return nil, invalid("Bad shape", "Fog either reveals or hides.")
	}
	if len(s.Fog) >= FogShapesMax {
		return nil, invalid("Too much fog", "This room already holds as many fog shapes as it can.")
	}
	if c.Kind == ShapeRect {
		if len(c.Points) != 4 {
			return nil, invalid("Bad shape", "A fog rectangle is two corners.")
		}
	}
	minPoints := 3
	if c.Kind == ShapeRect {
		minPoints = 2
	}
	if err := checkPoints("fog shape", c.Points, minPoints, FogPointsMax); err != nil {
		return nil, err
	}
	if err := s.fogBudget(len(c.Points)); err != nil {
		return nil, err
	}
	shape := FogShape{
		ID:      env.id(),
		LayerID: c.Layer,
		Kind:    c.Kind,
		Mode:    c.Mode,
		Points:  slices.Clone(c.Points),
	}
	s.Fog = append(s.Fog, shape)
	if !l.FogEnabled {
		l.FogEnabled = true
		l.FogPrefill = c.Mode == FogReveal
	}
	s.Normalize()
	return nil, nil
}

type FogRemove struct {
	ID ulid.ULID `json:"id"`
}

func (c *FogRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the fog")
}
func (c *FogRemove) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if !slices.ContainsFunc(s.Fog, func(f FogShape) bool { return f.ID == c.ID }) {
		return nil, notFound("Fog gone", "That fog shape is no longer there.")
	}
	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.ID == c.ID })
	s.Normalize()
	return nil, nil
}

type FogClear struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *FogClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the fog")
}
func (c *FogClear) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == c.Layer })
	s.Normalize()
	return nil, nil
}
