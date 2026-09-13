package room

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oklog/ulid/v2"
)

type HexNote struct {
	LayerID  ulid.ULID `json:"layerId"`
	Q        int       `json:"q"`
	R        int       `json:"r"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	Revealed bool      `json:"revealed"`
}
type NotesUpserted struct {
	Kind
	Notes []HexNote `json:"notes"`
}

func (*NotesUpserted) changeType() string { return "notes.upserted" }

type NotesRemoved struct {
	Kind
	Layer ulid.ULID `json:"layer"`
	Cells []Cell    `json:"cells"`
}

func (*NotesRemoved) changeType() string { return "notes.removed" }

func (s *State) Note(layer ulid.ULID, q, r int) *HexNote {
	for i := range s.Notes {
		if s.Notes[i].LayerID == layer && s.Notes[i].Q == q && s.Notes[i].R == r {
			return &s.Notes[i]
		}
	}
	return nil
}
func (s *State) ProjectedNote(layer ulid.ULID, q, r int, role Role) *HexNote {
	note := s.Note(layer, q, r)
	if note == nil || (role != RoleGM && !note.Revealed) {
		return nil
	}
	out := *note
	return &out
}
func (s *State) requireNote(layer ulid.ULID, q, r int) (*HexNote, error) {
	note := s.Note(layer, q, r)
	if note == nil {
		return nil, notFound("Nothing written here", "There is no note on that hex.")
	}
	return note, nil
}

type NoteSet struct {
	Layer ulid.ULID `json:"layer"`
	Q     int       `json:"q"`
	R     int       `json:"r"`
	Title string    `json:"title"`
	Body  string    `json:"body"`
}

func (c *NoteSet) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}
	if err := s.requirePlayerLayer(a, c.Layer); err != nil {
		return err
	}
	if held := s.Note(c.Layer, c.Q, c.R); held != nil && !held.Revealed {
		return forbidden("The GM has this hex", "The GM is keeping that hex to themselves. Ask them about it.")
	}
	return nil
}
func (c *NoteSet) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireLayer(c.Layer); err != nil {
		return nil, err
	}
	if err := checkCell(Cell{Q: c.Q, R: c.R}); err != nil {
		return nil, err
	}
	if err := checkNote(c.Title, c.Body); err != nil {
		return nil, err
	}
	held := s.Note(c.Layer, c.Q, c.R)
	was := 0
	if held != nil {
		was = noteBytes(*held)
	} else if len(s.Notes) >= NotesMax {
		return nil, invalid("Key full", fmt.Sprintf("This room already holds %d hex notes. Remove one before writing another.", NotesMax))
	}
	if err := s.noteBudget(len(c.Title) + len(c.Body) - was); err != nil {
		return nil, err
	}
	if held != nil {
		held.Title, held.Body = c.Title, c.Body
		s.Normalize()
		return nil, nil
	}
	s.Notes = append(s.Notes, HexNote{
		LayerID: c.Layer, Q: c.Q, R: c.R, Title: c.Title, Body: c.Body, Revealed: !a.GM(),
	})
	s.Normalize()
	return nil, nil
}

type NoteReveal struct {
	Layer    ulid.ULID `json:"layer"`
	Q        int       `json:"q"`
	R        int       `json:"r"`
	Revealed bool      `json:"revealed"`
}

func (c *NoteReveal) Authorize(s *State, a Actor) error {
	return requireGM(a, "share a hex note")
}
func (c *NoteReveal) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	note, err := s.requireNote(c.Layer, c.Q, c.R)
	if err != nil {
		return nil, err
	}
	note.Revealed = c.Revealed
	s.Normalize()
	return nil, nil
}

type NoteRemove struct {
	Layer ulid.ULID `json:"layer"`
	Q     int       `json:"q"`
	R     int       `json:"r"`
}

func (c *NoteRemove) Authorize(s *State, a Actor) error {
	return requireGM(a, "rub out a hex note")
}
func (c *NoteRemove) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if _, err := s.requireNote(c.Layer, c.Q, c.R); err != nil {
		return nil, err
	}
	s.Notes = slices.DeleteFunc(s.Notes, func(n HexNote) bool {
		return n.LayerID == c.Layer && n.Q == c.Q && n.R == c.R
	})
	s.Normalize()
	return nil, nil
}
func checkNote(title, body string) error {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(body) == "" {
		return invalid("Nothing to write", "A hex note needs a title or something in its body.")
	}
	if err := checkName("hex note", title); err != nil {
		return err
	}
	if len([]rune(body)) > NoteBodyLimit {
		return invalid("Note too long", fmt.Sprintf("A hex note is at most %d characters.", NoteBodyLimit))
	}
	return nil
}
func noteBytes(n HexNote) int { return len(n.Title) + len(n.Body) }
func (s *State) noteBudget(adding int) error {
	total := 0
	for _, n := range s.Notes {
		total += noteBytes(n)
	}
	if total+adding > NoteBytesBudget {
		return invalid("Key full", "This room holds as many hex notes as it can. Rub one out first.")
	}
	return nil
}
