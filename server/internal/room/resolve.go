package room

import (
	"context"
	"strings"

	"github.com/oklog/ulid/v2"
)

type Resolver interface {
	Resolve(ctx context.Context, lib Library, s *State) error
}
type Library interface {
	Map(ctx context.Context, asset ulid.ULID) (MapRef, error)
	Monster(ctx context.Context, id ulid.ULID) (MonsterInfo, error)
	Picture(ctx context.Context, id ulid.ULID, kind PictureKind) (PictureInfo, error)
	Track(ctx context.Context, id ulid.ULID) (TrackInfo, error)
	Character(ctx context.Context, id ulid.ULID) (CharacterInfo, error)
}
type PictureKind string

const (
	PictureToken  PictureKind = "token"
	PictureAvatar PictureKind = "avatar"
)

type MonsterInfo struct {
	Name            string
	Size            Size
	HP              int
	AC              int
	Image           string
	InitiativeBonus int
}
type PictureInfo struct {
	Name   string
	Image  string
	Width  int
	Height int
}
type CharacterInfo struct {
	ID              ulid.ULID
	OwnerID         ulid.ULID
	Name            string
	Size            Size
	HP              int
	MaxHP           int
	AC              int
	Image           string
	InitiativeBonus int
}

func CreatureSize(value string) Size {
	size := Size(strings.ToLower(strings.TrimSpace(value)))
	if !size.Valid() {
		return SizeMedium
	}
	return size
}
func gone(err error) bool {
	e, ok := err.(*Error)
	return ok && e.Code == CodeNotFound
}
func (s *State) seatFor(character ulid.ULID) *Player {
	for i := range s.Players {
		if s.Players[i].CharacterID != nil && *s.Players[i].CharacterID == character {
			return &s.Players[i]
		}
	}
	return nil
}
func (s *State) pawnFor(character ulid.ULID) *Pawn {
	for i := range s.Pawns {
		p := &s.Pawns[i]
		if p.Kind == PawnPlayer && p.CharacterID != nil && *p.CharacterID == character {
			return p
		}
	}
	return nil
}
func (s *State) unplacedSeats() []Player {
	out := make([]Player, 0, len(s.Players))
	for _, p := range s.Players {
		if p.Connected && p.CharacterID != nil && s.pawnFor(*p.CharacterID) == nil {
			out = append(out, clonePlayer(p))
		}
	}
	return out
}
func characterPawn(info CharacterInfo, seat *Player) Pawn {
	hp, maxHP, ac := info.HP, info.MaxHP, info.AC
	owner, character := info.OwnerID, info.ID
	image := info.Image
	if image == "" && seat != nil && seat.Avatar != DefaultAvatar {
		image = seat.Avatar
	}
	return Pawn{
		Name:        info.Name,
		Image:       image,
		Size:        info.Size,
		HP:          &hp,
		MaxHP:       &maxHP,
		AC:          &ac,
		OwnerID:     &owner,
		CharacterID: &character,
	}
}
func spawnName(typed, pictured string) string {
	if name := strings.TrimSpace(typed); name != "" {
		return name
	}
	return strings.TrimSpace(pictured)
}
