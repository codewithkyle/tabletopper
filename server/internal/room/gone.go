package room

import (
	"slices"
	"strings"

	"github.com/oklog/ulid/v2"
)

const imagePath = "/assets/images/"

func ImageURL(id ulid.ULID) string { return imagePath + id.String() }

func imageAsset(url string) (ulid.ULID, bool) {
	rest, ok := strings.CutPrefix(url, imagePath)
	if !ok {
		return ulid.ULID{}, false
	}
	rest, _, _ = strings.Cut(rest, "?")
	id, err := ulid.ParseStrict(rest)
	return id, err == nil
}

type CharacterGone struct {
	ID ulid.ULID `json:"id"`
}

func (c *CharacterGone) Authorize(s *State, a Actor) error { return nil }
func (c *CharacterGone) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	s.forgetCharacter(c.ID)
	s.Normalize()
	return nil, nil
}

type MonsterGone struct {
	ID ulid.ULID `json:"id"`
}

func (c *MonsterGone) Authorize(s *State, a Actor) error { return nil }
func (c *MonsterGone) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	s.forgetMonster(c.ID)
	s.Normalize()
	return nil, nil
}

type AssetGone struct {
	ID ulid.ULID `json:"id"`
}

func (c *AssetGone) Authorize(s *State, a Actor) error { return nil }
func (c *AssetGone) Apply(s *State, a Actor, env Env) ([]Signal, error) {
	s.forgetAsset(c.ID)
	s.Normalize()
	return nil, nil
}
func (s *State) forgetCharacter(id ulid.ULID) {
	for i := range s.Players {
		p := &s.Players[i]
		if p.CharacterID != nil && *p.CharacterID == id {
			p.CharacterID = nil
			p.CharacterName = ""
		}
	}
	s.dropPawns(func(p Pawn) bool { return p.CharacterID != nil && *p.CharacterID == id })
}
func (s *State) forgetMonster(id ulid.ULID) {
	for i := range s.Pawns {
		p := &s.Pawns[i]
		if p.MonsterID != nil && *p.MonsterID == id {
			p.MonsterID = nil
		}
	}
}
func (s *State) forgetAsset(id ulid.ULID) {
	for i := range s.Table.Layers {
		l := &s.Table.Layers[i]
		if l.Map != nil && l.Map.AssetID == id {
			l.Map = nil
		}
		if l.GMMap != nil && l.GMMap.AssetID == id {
			l.GMMap = nil
		}
	}
	s.dropArt(func(art TileArt) bool { return art.AssetID == id })
	image := ImageURL(id)
	for i := range s.Pawns {
		if s.Pawns[i].Image == image {
			s.Pawns[i].Image = ""
		}
	}
	if s.Music.TrackID != nil && *s.Music.TrackID == id {
		s.forgetTrack()
	}
}
func (s *State) forgetTrack() {
	s.Music = Music{Loop: s.Music.Loop}
}
func (s *State) dropPawns(gone func(Pawn) bool) {
	for _, p := range s.Pawns {
		if gone(p) {
			s.dropEntriesFor(p.ID)
		}
	}
	s.Pawns = slices.DeleteFunc(s.Pawns, gone)
}
func (s *State) dropArt(gone func(TileArt) bool) {
	dropped := map[ulid.ULID]bool{}
	for _, art := range s.Table.Palette {
		if gone(art) {
			dropped[art.ID] = true
		}
	}
	if len(dropped) == 0 {
		return
	}
	s.Table.Palette = slices.DeleteFunc(s.Table.Palette, gone)
	s.Tiles = slices.DeleteFunc(s.Tiles, func(t Tile) bool { return dropped[t.Art] })
}
