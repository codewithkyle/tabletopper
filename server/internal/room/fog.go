package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE FOG FAMILY.
//
// SHAPES ARE THE SOURCE OF TRUTH, NOT A MASK. A rectangle is four integers and
// the mask it produces is a megabyte, so the shapes are what travel and each
// client rasterises its own texture from them. That is also what makes fog
// trivially undoable -- removing a shape is removing a shape -- and what lets a
// client redraw the mask at whatever resolution its display wants.
//
// THEY ARE A COLLECTION RATHER THAN A SINGLETON, unlike the table and the
// tracker, because a well-explored dungeon holds hundreds of polygons and
// resending all of them every time the party opens a door is the one case where
// the singleton rule would cost something real.
//
// ORDER IS MEANING HERE. Shapes apply in slice order, so a hide drawn over a
// reveal covers it again; Normalize sorts pawns and strokes and deliberately
// leaves this collection in the order it was built.
//
// THE TWO FLAGS ARE NOT HERE. Enabled and prefill belong to the layer, and a
// change to either is a table.updated -- there is one place a layer's
// properties live and the fog commands write to it rather than duplicating it.

// FogAdded carries the whole new shape.
type FogAdded struct {
	Header
	Shape FogShape `json:"shape"`
}

func (*FogAdded) eventType() string { return "fog.added" }

// FogRemoved names one shape.
type FogRemoved struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*FogRemoved) eventType() string { return "fog.removed" }

// FogCleared empties one layer. It names the layer rather than listing the
// shapes because "clear the fog" on a well-explored floor would otherwise be a
// message carrying two thousand ids to say one thing.
type FogCleared struct {
	Header
	Layer ulid.ULID `json:"layer"`
}

func (*FogCleared) eventType() string { return "fog.cleared" }

// FogSetEnabled turns a layer's fog on or off.
type FogSetEnabled struct {
	Layer   ulid.ULID `json:"layer"`
	Enabled bool      `json:"enabled"`
}

func (c *FogSetEnabled) Authorize(s *State, a Actor) error {
	return requireGM(a, "turn fog on or off")
}

func (c *FogSetEnabled) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}

	// The shapes are kept when fog is switched off. A GM toggling fog to check
	// what the players can see has not thrown away an hour of revealing, and
	// clearing is its own command for the times they mean it.
	l.FogEnabled = c.Enabled
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// FogSetPrefill decides which way round a layer's fog works: prefilled means
// the floor starts covered and shapes reveal it, and the alternative is a clear
// floor that shapes cover.
type FogSetPrefill struct {
	Layer   ulid.ULID `json:"layer"`
	Prefill bool      `json:"prefill"`
}

func (c *FogSetPrefill) Authorize(s *State, a Actor) error {
	return requireGM(a, "change how a layer's fog works")
}

func (c *FogSetPrefill) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	l, err := s.requireLayer(c.Layer)
	if err != nil {
		return nil, err
	}

	l.FogPrefill = c.Prefill
	s.Normalize()

	return []Emission{tableUpdated(s)}, nil
}

// FogAdd draws one shape.
type FogAdd struct {
	Layer  ulid.ULID `json:"layer"`
	Kind   ShapeKind `json:"kind"`
	Mode   FogMode   `json:"mode"`
	Points []int     `json:"points"`
}

func (c *FogAdd) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the fog")
}

func (c *FogAdd) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
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

	// A RECTANGLE IS EXACTLY TWO CORNERS, not two or more. The renderer reads
	// four numbers out of it without checking, and a rectangle with a fifth
	// number is a client that thinks it is sending something else.
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
	s.Normalize()

	return []Emission{to(ToAll, &FogAdded{Shape: cloneShape(shape)})}, nil
}

// FogRemove takes one shape back off.
type FogRemove struct {
	ID ulid.ULID `json:"id"`
}

func (c *FogRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the fog")
}

func (c *FogRemove) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if !slices.ContainsFunc(s.Fog, func(f FogShape) bool { return f.ID == c.ID }) {
		return nil, notFound("Fog gone", "That fog shape is no longer there.")
	}

	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.ID == c.ID })
	s.Normalize()

	return []Emission{to(ToAll, &FogRemoved{ID: c.ID})}, nil
}

// FogClear empties one layer's fog.
type FogClear struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *FogClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the fog")
}

func (c *FogClear) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}

	s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == c.Layer })
	s.Normalize()

	return []Emission{to(ToAll, &FogCleared{Layer: c.Layer})}, nil
}
