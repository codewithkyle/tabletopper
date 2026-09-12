package room

import (
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

type StrokeBegan struct {
	Header
	Stroke Stroke `json:"stroke"`
}

func (*StrokeBegan) eventType() string { return "stroke.began" }

type StrokeExtended struct {
	Header
	ID     ulid.ULID `json:"id"`
	Points []int     `json:"points"`
}

func (*StrokeExtended) eventType() string { return "stroke.extended" }

type StrokeEnded struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*StrokeEnded) eventType() string { return "stroke.ended" }

type StrokeErased struct {
	Header
	IDs []ulid.ULID `json:"ids"`
}

func (*StrokeErased) eventType() string { return "stroke.erased" }

type StrokeBegin struct {
	ID     ulid.ULID  `json:"id"`
	Layer  ulid.ULID  `json:"layer"`
	Kind   StrokeKind `json:"kind"`
	Color  string     `json:"color"`
	Width  int        `json:"width"`
	Points []int      `json:"points"`
}

func (c *StrokeBegin) Authorize(s *State, a Actor) error {
	if err := s.requirePlayerLayer(a, c.Layer); err != nil {
		return err
	}
	if !a.GM() && !s.Table.PlayersCanDraw {
		return forbidden("Drawing is off", "The GM has turned off drawing for players.")
	}
	return nil
}
func (c *StrokeBegin) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if c.ID.Compare(ulid.ULID{}) == 0 {
		return nil, invalid("Bad stroke", "That stroke has no id.")
	}
	if s.Stroke(c.ID) != nil {
		return nil, invalid("Bad stroke", "A stroke with that id is already on the table.")
	}
	if !c.Kind.Valid() {
		return nil, invalid("Bad stroke", "That is not a kind of drawing.")
	}
	if len(s.Strokes) >= StrokesMax {
		return nil, invalid("Too many strokes", "This room already holds as many strokes as it can.")
	}
	if c.Width < 1 || c.Width > StrokeWidthMax {
		return nil, invalid("Bad stroke", fmt.Sprintf("A stroke is between 1 and %d pixels wide.", StrokeWidthMax))
	}
	if err := checkColor("stroke colour", c.Color); err != nil {
		return nil, err
	}
	if c.Kind.Shape() && len(c.Points) != 4 {
		return nil, invalid("Bad stroke", "That shape is two points.")
	}
	if err := checkPoints("stroke", c.Points, 1, StrokeChunkMax); err != nil {
		return nil, err
	}
	if c.Kind.Shape() && c.Points[0] == c.Points[2] && c.Points[1] == c.Points[3] {
		return nil, invalid("Bad stroke", "That shape has no size.")
	}
	if err := s.strokeBudget(a, len(c.Points)); err != nil {
		return nil, err
	}
	stroke := Stroke{
		ID:      c.ID,
		By:      a.ID,
		LayerID: c.Layer,
		Kind:    c.Kind,
		Color:   c.Color,
		Width:   c.Width,
		Points:  slices.Clone(c.Points),
		Done:    c.Kind.Shape(),
	}
	s.Strokes = append(s.Strokes, stroke)
	s.Normalize()
	return nil, nil
}

type StrokeExtend struct {
	ID     ulid.ULID `json:"id"`
	Points []int     `json:"points"`
}

func (c *StrokeExtend) Authorize(s *State, a Actor) error {
	return s.requireOwnStroke(a, c.ID)
}
func (c *StrokeExtend) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	st := s.Stroke(c.ID)
	if st == nil {
		return nil, notFound("Stroke gone", "That stroke is no longer on the table.")
	}
	if st.Done {
		return nil, invalid("Stroke finished", "That stroke has already been finished.")
	}
	if err := checkPoints("stroke", c.Points, 1, StrokeChunkMax); err != nil {
		return nil, err
	}
	if len(st.Points)+len(c.Points) > StrokePointsMax {
		return nil, invalid("Stroke too long", "That stroke has grown past what one line can hold.")
	}
	if err := s.strokeBudget(a, len(c.Points)); err != nil {
		return nil, err
	}
	st.Points = append(st.Points, c.Points...)
	s.Normalize()
	return nil, nil
}

type StrokeEnd struct {
	ID ulid.ULID `json:"id"`
}

func (c *StrokeEnd) Authorize(s *State, a Actor) error {
	return s.requireOwnStroke(a, c.ID)
}
func (c *StrokeEnd) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	st := s.Stroke(c.ID)
	if st == nil {
		return nil, notFound("Stroke gone", "That stroke is no longer on the table.")
	}
	st.Done = true
	s.Normalize()
	return nil, nil
}

type StrokeErase struct {
	IDs []ulid.ULID `json:"ids"`
}

func (c *StrokeErase) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}
	for _, id := range c.IDs {
		if err := s.requireOwnStroke(a, id); err != nil {
			return err
		}
	}
	return nil
}
func (c *StrokeErase) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}
	for _, id := range c.IDs {
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.ID == id })
	}
	s.Normalize()
	return nil, nil
}

type StrokeClear struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *StrokeClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the drawing")
}
func (c *StrokeClear) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == c.Layer })
	s.Normalize()
	return nil, nil
}
func (s *State) requireOwnStroke(a Actor, id ulid.ULID) error {
	st := s.Stroke(id)
	if st == nil {
		return notFound("Stroke gone", "That stroke is no longer on the table.")
	}
	if st.By != a.ID {
		return forbidden("Not your stroke", "You can only change a line you drew.")
	}
	return nil
}
