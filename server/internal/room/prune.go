package room

import (
	"context"

	"github.com/oklog/ulid/v2"
)

type cached[T any] struct {
	value T
	err   error
}

func lookup[T any](memo map[ulid.ULID]cached[T], id ulid.ULID, fetch func() (T, error)) (T, error) {
	if c, ok := memo[id]; ok {
		return c.value, c.err
	}
	value, err := fetch()
	memo[id] = cached[T]{value: value, err: err}
	return value, err
}

type pruner struct {
	lib            Library
	drop           func(error) bool
	missing        []string
	readMaps       map[ulid.ULID]cached[MapRef]
	readTerrain    map[ulid.ULID]cached[PictureInfo]
	readImages     map[ulid.ULID]cached[struct{}]
	readMonsters   map[ulid.ULID]cached[MonsterInfo]
	readCharacters map[ulid.ULID]cached[CharacterInfo]
	readTracks     map[ulid.ULID]cached[TrackInfo]
}

func newPruner(lib Library, drop func(error) bool) *pruner {
	return &pruner{
		lib:            lib,
		drop:           drop,
		readMaps:       map[ulid.ULID]cached[MapRef]{},
		readTerrain:    map[ulid.ULID]cached[PictureInfo]{},
		readImages:     map[ulid.ULID]cached[struct{}]{},
		readMonsters:   map[ulid.ULID]cached[MonsterInfo]{},
		readCharacters: map[ulid.ULID]cached[CharacterInfo]{},
		readTracks:     map[ulid.ULID]cached[TrackInfo]{},
	}
}
func refused(err error) bool {
	_, ok := err.(*Error)
	return ok
}
func (p *pruner) verdict(name string, err error) (bool, error) {
	refusal, ok := err.(*Error)
	if !ok {
		return false, err
	}
	if !p.drop(err) {
		return false, nil
	}
	p.missing = append(p.missing, name+": "+refusal.Message)
	return true, nil
}
func (p *pruner) maps(ctx context.Context, s *State) error {
	for i := range s.Table.Layers {
		l := &s.Table.Layers[i]
		for _, slot := range []**MapRef{&l.Map, &l.GMMap} {
			held := *slot
			if held == nil {
				continue
			}
			ref, err := lookup(p.readMaps, held.AssetID, func() (MapRef, error) { return p.lib.Map(ctx, held.AssetID) })
			if err != nil {
				drop, fatal := p.verdict(l.Name, err)
				if fatal != nil {
					return fatal
				}
				if drop {
					*slot = nil
				}
				continue
			}
			*slot = cloneRef(&ref)
		}
	}
	return nil
}
func (p *pruner) palette(ctx context.Context, s *State) error {
	gone := map[ulid.ULID]bool{}
	for i := range s.Table.Palette {
		art := &s.Table.Palette[i]
		info, err := lookup(p.readTerrain, art.AssetID, func() (PictureInfo, error) { return p.lib.Picture(ctx, art.AssetID, PictureTerrain) })
		if err != nil {
			drop, fatal := p.verdict(art.Name, err)
			if fatal != nil {
				return fatal
			}
			if drop {
				gone[art.ID] = true
			}
			continue
		}
		art.Name, art.Image = info.Name, info.Image
	}
	s.dropArt(func(art TileArt) bool { return gone[art.ID] })
	return nil
}
func (p *pruner) pawns(ctx context.Context, s *State) error {
	gone := map[ulid.ULID]bool{}
	for i := range s.Pawns {
		pawn := &s.Pawns[i]
		if pawn.CharacterID != nil {
			character := *pawn.CharacterID
			if _, err := lookup(p.readCharacters, character, func() (CharacterInfo, error) { return p.lib.Character(ctx, character) }); err != nil {
				drop, fatal := p.verdict(pawn.Name, err)
				if fatal != nil {
					return fatal
				}
				if drop {
					gone[pawn.ID] = true
					continue
				}
			}
		}
		if pawn.MonsterID != nil {
			monster := *pawn.MonsterID
			if _, err := lookup(p.readMonsters, monster, func() (MonsterInfo, error) { return p.lib.Monster(ctx, monster) }); err != nil {
				drop, fatal := p.verdict(pawn.Name, err)
				if fatal != nil {
					return fatal
				}
				if drop {
					pawn.MonsterID = nil
				}
			}
		}
		if id, ok := imageAsset(pawn.Image); ok {
			if _, err := lookup(p.readImages, id, func() (struct{}, error) { return struct{}{}, p.lib.Image(ctx, id) }); err != nil {
				drop, fatal := p.verdict(pawn.Name, err)
				if fatal != nil {
					return fatal
				}
				if drop {
					pawn.Image = ""
				}
			}
		}
	}
	s.dropPawns(func(q Pawn) bool { return gone[q.ID] })
	return nil
}
func (p *pruner) seats(ctx context.Context, s *State) error {
	for i := range s.Players {
		seat := &s.Players[i]
		if seat.CharacterID == nil {
			continue
		}
		character := *seat.CharacterID
		_, err := lookup(p.readCharacters, character, func() (CharacterInfo, error) { return p.lib.Character(ctx, character) })
		if err == nil {
			continue
		}
		drop, fatal := p.verdict(seat.Name, err)
		if fatal != nil {
			return fatal
		}
		if drop {
			seat.CharacterID = nil
			seat.CharacterName = ""
		}
	}
	return nil
}
func (p *pruner) music(ctx context.Context, s *State) error {
	if s.Music.TrackID == nil {
		return nil
	}
	track := *s.Music.TrackID
	_, err := lookup(p.readTracks, track, func() (TrackInfo, error) { return p.lib.Track(ctx, track) })
	if err == nil {
		return nil
	}
	drop, fatal := p.verdict(s.Music.Name, err)
	if fatal != nil {
		return fatal
	}
	if drop {
		s.forgetTrack()
	}
	return nil
}
func Prune(ctx context.Context, lib Library, s *State) ([]string, error) {
	pruned := s.Clone()
	p := newPruner(lib, gone)
	for _, step := range []func(context.Context, *State) error{p.maps, p.palette, p.pawns, p.seats, p.music} {
		if err := step(ctx, &pruned); err != nil {
			return nil, err
		}
	}
	pruned.Normalize()
	*s = pruned
	return p.missing, nil
}
