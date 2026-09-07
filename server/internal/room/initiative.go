package room

import (
	"slices"

	"github.com/oklog/ulid/v2"
)

// THE INITIATIVE FAMILY. The tracker is a singleton, so all three commands end
// with the whole thing rather than with the line that changed.
//
// THE SLICE ORDER IS THE TURN ORDER. Nothing sorts by the Initiative field, and
// that field is informational: two creatures that rolled a 14 act in whichever
// order the GM dragged them into, and a stored order is the only representation
// that can hold the result of that drag. It is also why Normalize leaves this
// collection alone where it sorts pawns and players.
//
// "YOUR TURN" IS NOT AN EVENT. The client works it out when the active entry
// becomes one of its own pawns, which means the notification cannot disagree
// with the tracker beside it.

// InitiativeUpdated carries the whole tracker.
//
// IT CARRIES BOTH AUDIENCES' COPIES for the same reason PawnMoved does: it goes
// to everybody, and a hidden pawn's line is not in the players' order. The
// filtered copy is nil when nothing is hidden, and then both roles get the same
// object.
type InitiativeUpdated struct {
	Header
	Initiative Initiative `json:"initiative"`

	player *Initiative
}

func (*InitiativeUpdated) eventType() string { return "initiative.updated" }

func (e *InitiativeUpdated) ForRole(role Role) Event {
	if role == RoleGM || e.player == nil {
		return e
	}

	c := *e
	c.Initiative = *e.player

	return &c
}

// initiativeUpdated is the ToAll emission, with the players' copy attached.
func initiativeUpdated(s *State) []Emission {
	player := projectInitiative(s)

	return []Emission{to(ToAll, &InitiativeUpdated{
		Initiative: cloneInitiative(s.Initiative),
		player:     &player,
	})}
}

// projectInitiative is the players' tracker.
//
// AN ENTRY FOR A PAWN ON ANOTHER FLOOR STAYS, and an entry for a pawn the GM
// has hidden goes. Those are different facts: a creature that walked downstairs
// still has a turn and the players know it exists, where a hidden creature is
// one they have not met, and a line in the tracker naming it would be the
// giveaway that the hiding exists to prevent.
func projectInitiative(s *State) Initiative {
	in := cloneInitiative(s.Initiative)

	entries := make([]InitiativeEntry, 0, len(in.Entries))
	for _, e := range in.Entries {
		if e.PawnID != nil {
			p := s.Pawn(*e.PawnID)
			if p == nil || !p.Visible {
				continue
			}
		}
		entries = append(entries, e)
	}
	in.Entries = entries

	if in.Active != nil && !hasEntry(entries, *in.Active) {
		in.Active = nil
	}

	return in
}

// hasEntryFor reports whether the tracker names this pawn, which is how the
// visibility commands know whether hiding it changed a second thing on the
// players' screens.
func (s *State) hasEntryFor(pawn ulid.ULID) bool {
	for _, e := range s.Initiative.Entries {
		if e.PawnID != nil && *e.PawnID == pawn {
			return true
		}
	}

	return false
}

// dropEntriesFor removes every line naming a pawn that is being deleted, and
// reports whether it removed any.
//
// DELETING THE CREATURE WHOSE TURN IT IS ADVANCES THE TURN, wrapping, rather
// than leaving the tracker pointed at nothing. A GM who kills the goblin that
// is currently acting means the fight to carry on with the next combatant, and
// an empty active would make them press next to get there.
func (s *State) dropEntriesFor(pawn ulid.ULID) bool {
	names := func(e InitiativeEntry) bool { return e.PawnID != nil && *e.PawnID == pawn }

	if !slices.ContainsFunc(s.Initiative.Entries, names) {
		return false
	}

	active := -1
	if s.Initiative.Active != nil {
		active = slices.IndexFunc(s.Initiative.Entries, func(e InitiativeEntry) bool {
			return e.ID == *s.Initiative.Active
		})
	}

	// The successor is chosen from the old order, before the deletion, because
	// "the next combatant" is a fact about the order the GM built.
	var successor *ulid.ULID
	if active >= 0 && names(s.Initiative.Entries[active]) {
		n := len(s.Initiative.Entries)
		for i := 1; i < n; i++ {
			e := s.Initiative.Entries[(active+i)%n]
			if !names(e) {
				successor = &e.ID

				break
			}
		}
		s.Initiative.Active = successor
	}

	s.Initiative.Entries = slices.DeleteFunc(s.Initiative.Entries, names)
	if len(s.Initiative.Entries) == 0 {
		s.Initiative.Active = nil
		s.Initiative.Round = 0
	}

	return true
}

// InitiativeSet replaces the tracker. Reordering a line, adding one, renaming
// one and deleting one are all this command, because all four are the same
// gesture in a list the GM is editing directly.
type InitiativeSet struct {
	Entries []InitiativeEntry `json:"entries"`
	Active  *ulid.ULID        `json:"active"`
}

func (c *InitiativeSet) Authorize(s *State, a Actor) error {
	return requireGM(a, "change the initiative order")
}

func (c *InitiativeSet) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(c.Entries) > InitiativeMax {
		return nil, invalid("Too many entries", "The tracker holds at most 200 entries.")
	}

	entries := make([]InitiativeEntry, 0, len(c.Entries))
	seen := map[ulid.ULID]bool{}

	for _, e := range c.Entries {
		if err := checkRequiredName("tracker entry", e.Name); err != nil {
			return nil, err
		}
		if e.PawnID != nil {
			if _, err := s.requirePawn(*e.PawnID); err != nil {
				return nil, err
			}
		}
		if e.ID.Compare(ulid.ULID{}) == 0 {
			e.ID = env.id()
		}
		if seen[e.ID] {
			return nil, invalid("Duplicate entry", "The same tracker entry appears twice.")
		}
		seen[e.ID] = true

		entries = append(entries, e)
	}

	if c.Active != nil && !hasEntry(entries, *c.Active) {
		return nil, invalid("Bad turn", "The active turn is not one of the entries.")
	}

	// THE ROUND IS NOT RESET. Editing the order mid-fight -- a reinforcement
	// arriving, a mistake corrected -- is not the fight starting again, and the
	// round number is what the party's spell durations are counted in.
	s.Initiative.Entries = entries
	s.Initiative.Active = cloneID(c.Active)
	s.Normalize()

	return initiativeUpdated(s), nil
}

// InitiativeNext advances the turn.
type InitiativeNext struct{}

// Authorize lets the player whose turn it is end it. That is the whole reason
// this is not GM-only: passing the tablet back to the GM to press next is the
// friction the button exists to remove.
func (c *InitiativeNext) Authorize(s *State, a Actor) error {
	if a.GM() {
		return nil
	}

	if s.Initiative.Active != nil {
		if e := s.entry(*s.Initiative.Active); e != nil && e.PawnID != nil {
			if p := s.Pawn(*e.PawnID); p != nil && p.OwnerID != nil && *p.OwnerID == a.ID {
				return nil
			}
		}
	}

	return forbidden("Not your turn", "Only the GM, or whoever's turn it is, can advance the tracker.")
}

func (c *InitiativeNext) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(s.Initiative.Entries) == 0 {
		return nil, invalid("Nothing to advance", "There is nothing in the initiative tracker.")
	}

	from := -1
	if s.Initiative.Active != nil {
		from = slices.IndexFunc(s.Initiative.Entries, func(e InitiativeEntry) bool {
			return e.ID == *s.Initiative.Active
		})
	}

	var next int
	switch {
	case from < 0:
		// The first press of the button. Round one begins and nobody's turn
		// just ended, so only the start-of-turn conditions tick.
		next = 0
		s.Initiative.Round = 1
	case from+1 >= len(s.Initiative.Entries):
		next = 0
		s.Initiative.Round++
	default:
		next = from + 1
	}

	// CONDITIONS TICK ON THE TWO PAWNS AT THE SEAM, which is the 5e rule spelled
	// out: "until the end of your next turn" counts down as your turn ends, and
	// "until the start of your next turn" as it begins. A duration of -1 never
	// counts and never expires -- it is there until somebody takes it off.
	touched := map[ulid.ULID]bool{}
	if from >= 0 {
		if id, ok := s.tick(s.Initiative.Entries[from], ClearEnd); ok {
			touched[id] = true
		}
	}
	if id, ok := s.tick(s.Initiative.Entries[next], ClearStart); ok {
		touched[id] = true
	}

	// A copy, not a pointer into the slice. Active is a *ulid.ULID, and one
	// that pointed at an element of Entries would quietly follow that element
	// wherever a later edit moved it.
	active := s.Initiative.Entries[next].ID
	s.Initiative.Active = &active
	s.Normalize()

	out := initiativeUpdated(s)
	for _, p := range s.Pawns {
		if touched[p.ID] {
			out = append(out, s.pawnUpdated(p.ID)...)
		}
	}

	return out, nil
}

// tick counts one pawn's conditions down at one end of its turn and reports the
// pawn it changed, if it changed one.
func (s *State) tick(e InitiativeEntry, when ClearTrigger) (ulid.ULID, bool) {
	if e.PawnID == nil {
		return ulid.ULID{}, false
	}

	p := s.Pawn(*e.PawnID)
	if p == nil {
		return ulid.ULID{}, false
	}

	kept := make([]Condition, 0, len(p.Conditions))
	changed := false

	for _, cond := range p.Conditions {
		if cond.Clear != when || cond.Duration < 0 {
			kept = append(kept, cond)

			continue
		}

		changed = true
		cond.Duration--
		if cond.Duration > 0 {
			kept = append(kept, cond)
		}
	}

	if !changed {
		return ulid.ULID{}, false
	}

	p.Conditions = kept

	return p.ID, true
}

// entry finds one line of the tracker by id.
func (s *State) entry(id ulid.ULID) *InitiativeEntry {
	for i := range s.Initiative.Entries {
		if s.Initiative.Entries[i].ID == id {
			return &s.Initiative.Entries[i]
		}
	}

	return nil
}

// InitiativeClear empties the tracker, which is what the end of a fight is.
type InitiativeClear struct{}

func (c *InitiativeClear) Authorize(s *State, a Actor) error {
	return requireGM(a, "clear the initiative tracker")
}

func (c *InitiativeClear) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	s.Initiative = Initiative{}
	s.Normalize()

	return initiativeUpdated(s), nil
}
