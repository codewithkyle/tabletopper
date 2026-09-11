package room

import "github.com/oklog/ulid/v2"









type Pinged struct {
	Header
	Layer ulid.ULID `json:"layer"`
	X     int       `json:"x"`
	Y     int       `json:"y"`
}

func (*Pinged) eventType() string { return "pinged" }
func (*Pinged) Transient() bool   { return true }


type Ping struct {
	Layer ulid.ULID `json:"layer"`
	X     int       `json:"x"`
	Y     int       `json:"y"`
}



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

	
	
	
	
	
	
	
	
	return []Emission{to(ToAll, &Pinged{Layer: c.Layer, X: c.X, Y: c.Y})}, nil
}
