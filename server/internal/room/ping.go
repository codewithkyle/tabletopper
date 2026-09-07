package room

import "github.com/oklog/ulid/v2"

// Pinged is somebody pointing at the map. It is transient -- it fades on its
// own and there is nothing about it to restore on reconnect -- and it is the
// smallest complete example of the command-and-event split: a client asks to
// point, and everybody including the asker is told that somebody pointed.
//
// WHO PINGED IS IN THE HEADER, not in the payload. The hub stamps `by` on every
// event a player caused, and a ping is only useful with a name attached, so
// duplicating the id in a field here would be two places to keep in step.
type Pinged struct {
	Header
	Layer ulid.ULID `json:"layer"`
	X     int       `json:"x"`
	Y     int       `json:"y"`
}

func (*Pinged) eventType() string { return "pinged" }
func (*Pinged) Transient() bool   { return true }

// Ping points at a spot on a layer.
type Ping struct {
	Layer ulid.ULID `json:"layer"`
	X     int       `json:"x"`
	Y     int       `json:"y"`
}

// Authorize lets anybody ping, which is the point of the feature: it is how a
// player says "that door" without being able to move anything.
func (c *Ping) Authorize(s *State, a Actor) error {
	return s.requirePlayerLayer(a, c.Layer)
}

func (c *Ping) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if err := checkCoord("ping", c.X); err != nil {
		return nil, err
	}
	if err := checkCoord("ping", c.Y); err != nil {
		return nil, err
	}

	// IT GOES TO EVERYBODY, THE PINGER INCLUDED. Drawing your own ping locally
	// and everybody else's from the wire would put your marker on the map a
	// round trip before theirs, and the one thing a ping has to be is in the
	// same place at the same time on every screen.
	//
	// THE LAYER RIDES ALONG so that a GM working on another floor is not shown
	// a marker floating over a map it does not belong to. The client checks the
	// layer against the one it is drawing and drops the rest.
	return []Emission{to(ToAll, &Pinged{Layer: c.Layer, X: c.X, Y: c.Y})}, nil
}
