package room

import (
	"context"
	"slices"

	"github.com/oklog/ulid/v2"
)

func (s *State) ExportScene() ([]byte, error) {
	c := s.Clone()
	c.Seq = 0
	c.Room = RoomInfo{}
	c.Players = nil
	c.Initiative = Initiative{}
	c.Rolls = nil
	c.Music = Music{}
	c.Pawns = slices.DeleteFunc(c.Pawns, func(p Pawn) bool { return p.Kind == PawnPlayer })
	for i := range c.Pawns {
		c.Pawns[i].OwnerID = nil
		c.Pawns[i].CharacterID = nil
	}
	c.Strokes = slices.DeleteFunc(c.Strokes, func(st Stroke) bool { return !st.Done })
	for i := range c.Strokes {
		c.Strokes[i].By = ulid.ULID{}
	}
	for i := range c.Tiles {
		c.Tiles[i].By = ulid.ULID{}
	}
	return Marshal(&c)
}
func (s *State) ImportScene(from *State) {
	s.Table.Layers = cloneLayers(from.Table.Layers)
	s.Table.ActiveLayer = from.Table.ActiveLayer
	s.Table.Grid = from.Table.Grid
	s.Fog = make([]FogShape, len(from.Fog))
	for i, f := range from.Fog {
		s.Fog[i] = cloneShape(f)
	}
	s.Strokes = make([]Stroke, len(from.Strokes))
	for i, st := range from.Strokes {
		s.Strokes[i] = cloneStroke(st)
	}
	s.Pawns = make([]Pawn, len(from.Pawns))
	for i, p := range from.Pawns {
		s.Pawns[i] = clonePawn(p)
	}
	s.Table.Palette = cloneSlice(from.Table.Palette)
	s.Tiles = slices.Clone(from.Tiles)
	s.Initiative = Initiative{}
	if s.Layer(s.Table.ActiveLayer) == nil && len(s.Table.Layers) > 0 {
		s.Table.ActiveLayer = s.Table.Layers[0].ID
	}
	s.Normalize()
}

type SceneLoad struct {
	Resync
	Scene   *State   `json:"-"`
	Missing []string `json:"-"`
}

func (c *SceneLoad) Authorize(s *State, a Actor) error {
	return requireGM(a, "open a scene")
}
func (c *SceneLoad) Resolve(ctx context.Context, lib Library, s *State) error {
	if c.Scene == nil {
		return nil
	}
	c.Missing = nil
	p := newPruner(lib, refused)
	for _, step := range []func(context.Context, *State) error{p.maps, p.palette, p.pawns} {
		if err := step(ctx, c.Scene); err != nil {
			return err
		}
	}
	c.Missing = p.missing
	return nil
}
func (c *SceneLoad) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	if c.Scene == nil {
		return nil, notFound("Scene gone", "That scene could not be read.")
	}
	s.ImportScene(c.Scene)
	return []Signal{signal(ToAll, &TableCleared{})}, nil
}
