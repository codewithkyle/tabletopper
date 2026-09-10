package room

import (
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE STROKE FAMILY: drawing on a layer, freehand or as a shape.
//
// A STROKE IS A LINE OR A SHAPE AND THE KIND IS WHAT SAYS WHICH. Only a free
// stroke grows, which is why only a free stroke is extended; see StrokeBegin
// and StrokeKind.
//
// THE CLIENT MINTS THE ID. It has to: it begins a stroke and then sends chunks
// of it at roughly ten hertz, and it cannot wait for a round trip to learn what
// to call the thing its hand is already drawing. The server's side of that
// bargain is narrow and it is the whole of the trust extended -- the id must
// parse as a ULID and must not already be in use, and everything else about the
// stroke is validated exactly as if the server had made it.
//
// EXTEND IS THE THIRD AND LAST HOT PATH. It carries the run of new points and
// not the whole polyline, which is the one place a stroke event is a delta.
// A begin and an end carry entities.
//
// STROKES ARE STORED, so any texture a client rasterises them into is a cache
// and never the truth. Undo is removing the last stroke.

// StrokeBegan carries the whole stroke as it stood when it started, which is
// usually one or two points.
type StrokeBegan struct {
	Header
	Stroke Stroke `json:"stroke"`
}

func (*StrokeBegan) eventType() string { return "stroke.began" }

// StrokeExtended is the run of new points, appended to what the client already
// has. It is the only stroke event that is not idempotent.
type StrokeExtended struct {
	Header
	ID     ulid.ULID `json:"id"`
	Points []int     `json:"points"`
}

func (*StrokeExtended) eventType() string { return "stroke.extended" }

// StrokeEnded closes a stroke. A client that has been appending points can stop
// expecting more.
type StrokeEnded struct {
	Header
	ID ulid.ULID `json:"id"`
}

func (*StrokeEnded) eventType() string { return "stroke.ended" }

// StrokeErased names the strokes a pass of the eraser took out. It is a list
// because one drag of an eraser crosses several lines and one event is one
// re-render.
type StrokeErased struct {
	Header
	IDs []ulid.ULID `json:"ids"`
}

func (*StrokeErased) eventType() string { return "stroke.erased" }

// StrokeCleared empties one layer.
type StrokeCleared struct {
	Header
	Layer ulid.ULID `json:"layer"`
}

func (*StrokeCleared) eventType() string { return "stroke.cleared" }

// StrokeBegin starts a line, or places a whole shape.
//
// A SHAPE NEVER STREAMS, AND THAT IS FORCED BY stroke.extend RATHER THAN
// CHOSEN. Extend APPENDS -- it is the one delta in this protocol -- and a
// rubber-banded rectangle changes its SECOND corner on every frame, which an
// append cannot express. So the client sends a shape once, when the button
// comes up, and it arrives complete: Apply marks it Done, and the extend that
// would grow it is refused by the rule that already refuses a finished stroke.
//
// The cost is that other people see a shape appear whole instead of watching it
// grow, and the alternative was a transient preview event -- a fourth hot path
// for a gesture that lasts a second, to animate something nobody at a table
// would notice was missing.
type StrokeBegin struct {
	ID     ulid.ULID  `json:"id"`
	Layer  ulid.ULID  `json:"layer"`
	Kind   StrokeKind `json:"kind"`
	Color  string     `json:"color"`
	Width  int        `json:"width"`
	Points []int      `json:"points"`
}

// Authorize reads the room's own setting rather than the role. Players drawing
// is the ordinary case -- somebody sketching the plan on the tavern table -- so
// it is on by default, and a GM who does not want it turns it off once.
func (c *StrokeBegin) Authorize(s *State, a Actor) error {
	if err := s.requirePlayerLayer(a, c.Layer); err != nil {
		return err
	}
	if !a.GM() && !s.Table.PlayersCanDraw {
		return forbidden("Drawing is off", "The GM has turned off drawing for players.")
	}

	return nil
}

func (c *StrokeBegin) Apply(s *State, a Actor, env Env) ([]Emission, error) {
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

	// A SHAPE IS EXACTLY TWO POINTS, not two or more. The client reads four
	// numbers out of one without checking -- a rectangle's opposite corners, a
	// circle's centre and rim, a cone's apex and base midpoint -- and a fifth
	// number is a client that thinks it is sending something else. It is the
	// rule FogAdd applies to a rectangle, for the same reason.
	if c.Kind.Shape() && len(c.Points) != 4 {
		return nil, invalid("Bad stroke", "That shape is two points.")
	}
	if err := checkPoints("stroke", c.Points, 1, StrokeChunkMax); err != nil {
		return nil, err
	}

	// AND A SHAPE OF NO SIZE IS A CLICK RATHER THAN A DRAG. The client already
	// drops one; this is the server not taking its word for it, because a
	// circle whose rim is its centre draws nothing and can never be pointed at
	// to be rubbed out.
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

	return []Emission{to(ToAll, &StrokeBegan{Stroke: cloneStroke(stroke)})}, nil
}

// StrokeExtend appends the next run of points.
type StrokeExtend struct {
	ID     ulid.ULID `json:"id"`
	Points []int     `json:"points"`
}

func (c *StrokeExtend) Authorize(s *State, a Actor) error {
	return s.requireOwnStroke(a, c.ID)
}

func (c *StrokeExtend) Apply(s *State, a Actor, env Env) ([]Emission, error) {
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

	return []Emission{to(ToAll, &StrokeExtended{ID: c.ID, Points: slices.Clone(c.Points)})}, nil
}

// StrokeEnd closes a line. It is idempotent in effect but not in permission: a
// stroke that is already done still belongs to the same person.
type StrokeEnd struct {
	ID ulid.ULID `json:"id"`
}

func (c *StrokeEnd) Authorize(s *State, a Actor) error {
	return s.requireOwnStroke(a, c.ID)
}

func (c *StrokeEnd) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	st := s.Stroke(c.ID)
	if st == nil {
		return nil, notFound("Stroke gone", "That stroke is no longer on the table.")
	}

	st.Done = true
	s.Normalize()

	return []Emission{to(ToAll, &StrokeEnded{ID: c.ID})}, nil
}

// StrokeErase removes strokes. The GM can rub out anybody's; a player can only
// rub out their own, which is the same rule as owning a pawn and exists for the
// same reason.
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

func (c *StrokeErase) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if err := checkSelection(len(c.IDs)); err != nil {
		return nil, err
	}

	// Only the ids that were actually there are echoed. An eraser dragged over
	// a line two people rubbed out at once should not tell everybody to remove
	// something twice.
	erased := make([]ulid.ULID, 0, len(c.IDs))
	for _, id := range c.IDs {
		if s.Stroke(id) == nil {
			continue
		}
		erased = append(erased, id)
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.ID == id })
	}
	s.Normalize()

	if len(erased) == 0 {
		return nil, nil
	}

	return []Emission{to(ToAll, &StrokeErased{IDs: erased})}, nil
}

// StrokeClear wipes one layer's drawing.
type StrokeClear struct {
	Layer ulid.ULID `json:"layer"`
}

func (c *StrokeClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the drawing")
}

func (c *StrokeClear) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}

	s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == c.Layer })
	s.Normalize()

	return []Emission{to(ToAll, &StrokeCleared{Layer: c.Layer})}, nil
}

// requireOwnStroke is the authority rule the three per-stroke commands share.
// Note that it is own-stroke for the GM too: extending somebody else's line is
// not a thing a GM wants to do, because the person drawing it is still drawing
// it, and erasing is the command for taking it away.
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
