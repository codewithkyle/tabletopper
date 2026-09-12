package hub

import (
	"slices"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func (a *actor) changed(before *room.State, changes []room.Change) {
	for _, ch := range changes {
		switch c := ch.(type) {
		case *room.PawnsUpserted:
			for _, p := range c.Pawns {
				if before.Pawn(p.ID) == nil {
					a.remember(p)
					continue
				}
				a.writeThrough(p)
			}
		case *room.PlayersRemoved:
			for _, id := range c.IDs {
				delete(a.secret, id)
				if _, kicked := a.kicked[id]; kicked {
					continue
				}
				a.drop(id, reasonLeft)
			}
		}
	}
}
func (a *actor) keepSecret(user ulid.ULID, roll room.Roll) {
	kept := append(a.secret[user], roll)
	if len(kept) > room.SecretRollsMax {
		kept = slices.Delete(kept, 0, len(kept)-room.SecretRollsMax)
	}
	a.secret[user] = kept
}
func (a *actor) remember(p room.Pawn) {
	if character, hp, owed := writeThroughHP(p); owed {
		a.lastHP[character] = hp
	}
}
func (a *actor) writeThrough(p room.Pawn) {
	character, hp, owed := writeThroughHP(p)
	if !owed || a.sheet == nil {
		return
	}
	if last, known := a.lastHP[character]; known && last == hp {
		return
	}
	a.lastHP[character] = hp
	a.sheet.put(character, hp)
}
func writeThroughHP(p room.Pawn) (ulid.ULID, int, bool) {
	if p.Kind != room.PawnPlayer || p.CharacterID == nil || p.HP == nil {
		return ulid.ULID{}, 0, false
	}
	return *p.CharacterID, *p.HP, true
}
