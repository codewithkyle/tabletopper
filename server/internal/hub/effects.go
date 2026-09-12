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
	if v, ok := room.PawnVitals(p); ok {
		a.lastSheet[*p.CharacterID] = v
	}
}
func (a *actor) writeThrough(p room.Pawn) {
	v, ok := room.PawnVitals(p)
	if !ok || a.sheet == nil {
		return
	}
	character := *p.CharacterID
	if last, known := a.lastSheet[character]; known && last == v {
		return
	}
	a.lastSheet[character] = v
	a.sheet.put(character, v)
}
