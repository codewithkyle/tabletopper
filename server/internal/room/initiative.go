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
//
// THE GATE IS Visible AND NOT Shown, deliberately, and it is the same gate
// ProjectedInitiative applies one loop later. Shown asks about the active layer
// as well, which is exactly the question the paragraph above answers no to.
//
// HIDING ONE OF NINE GOBLINS TAKES A PIP OFF THE GROUP AND LEAVES THE LINE. A
// group is filtered member by member, and only an entry emptied by that
// filtering is dropped -- so the players' count is the count of what they can
// see, which is the whole of what the group card is telling them.
func projectInitiative(s *State) Initiative {
	in := cloneInitiative(s.Initiative)

	entries := make([]InitiativeEntry, 0, len(in.Entries))
	for _, e := range in.Entries {
		if len(e.PawnIDs) > 0 {
			kept := make([]ulid.ULID, 0, len(e.PawnIDs))
			for _, id := range e.PawnIDs {
				if p := s.Pawn(id); p != nil && p.Visible {
					kept = append(kept, id)
				}
			}
			if len(kept) == 0 {
				continue
			}
			e.PawnIDs = kept
		}
		entries = append(entries, e)
	}
	in.Entries = entries

	if in.Active != nil && !hasEntry(entries, *in.Active) {
		in.Active = nil
	}

	return in
}

// ProjectedInitiative is the tracker this role may see and the pawns it names,
// computed in one pass so that the two cannot drift.
//
// THE PAWNS ARE HANDED BACK WITH THE TRACKER BECAUSE THE ALTERNATIVE HAS A BUG
// IN IT. A caller that fetched the tracker and then asked Project(role) for the
// pawns would be applying two different filters: Project runs a player's pawns
// through Shown, which gates on the active layer, and the tracker deliberately
// keeps the entry of a creature that walked downstairs. The shape of that
// disagreement is a player watching the party member who went up the stairs
// turn into a nameless line with no portrait for the rest of the fight -- which
// is precisely the case the tracker's own rule exists to serve.
//
// THE MAP IS KEYED BY ID because the caller looks up one pawn per member and a
// slice would be a scan per pip.
func (s *State) ProjectedInitiative(role Role) (Initiative, map[ulid.ULID]Pawn) {
	in := cloneInitiative(s.Initiative)
	if role != RoleGM {
		in = projectInitiative(s)
	}

	pawns := map[ulid.ULID]Pawn{}
	for _, e := range in.Entries {
		for _, id := range e.PawnIDs {
			if _, done := pawns[id]; done {
				continue
			}

			p := s.Pawn(id)
			if p == nil {
				continue
			}

			if role == RoleGM {
				pawns[id] = clonePawn(*p)

				continue
			}

			// Visible alone, for the reason above. projectInitiative has
			// already dropped everything else from the entries, so this is the
			// same test twice rather than a second rule.
			if !p.Visible {
				continue
			}

			pawns[id] = projectPawn(clonePawn(*p), s.Table)
		}
	}

	return in, pawns
}

// hasEntryFor reports whether the tracker names this pawn, which is how the
// visibility commands know whether hiding it changed a second thing on the
// players' screens.
func (s *State) hasEntryFor(pawn ulid.ULID) bool {
	for _, e := range s.Initiative.Entries {
		if slices.Contains(e.PawnIDs, pawn) {
			return true
		}
	}

	return false
}

// dropEntriesFor takes a pawn that is being deleted out of every line that
// names it, and reports whether it changed anything.
//
// A GROUP LOSES A MEMBER RATHER THAN THE LINE. Killing one of nine goblins and
// taking it off the table leaves eight goblins with a turn; only the line whose
// LAST pawn has gone goes with it, which is the same rule projectInitiative
// applies to a hidden member.
//
// DELETING THE CREATURE WHOSE TURN IT IS ADVANCES THE TURN, wrapping, rather
// than leaving the tracker pointed at nothing. A GM who kills the goblin that
// is currently acting means the fight to carry on with the next combatant, and
// an empty active would make them press next to get there.
func (s *State) dropEntriesFor(pawn ulid.ULID) bool {
	if !s.hasEntryFor(pawn) {
		return false
	}

	return s.dropMembers(func(id ulid.ULID) bool { return id == pawn })
}

// dropMembers is dropped applied to the live tracker, and it reports whether
// anything moved.
func (s *State) dropMembers(gone func(ulid.ULID) bool) bool {
	entries, active, changed := dropped(s.Initiative.Entries, s.Initiative.Active, gone)

	s.Initiative.Entries = entries
	s.Initiative.Active = active

	if len(entries) == 0 {
		if s.Initiative.Active != nil || s.Initiative.Round != 0 {
			changed = true
		}
		s.Initiative.Active = nil
		s.Initiative.Round = 0
	}

	return changed
}

// dropped is the shared half of every removal: take out the members the
// predicate names, drop a line that had pawns and has none left, and move the
// turn off a line that is going.
//
// IT IS A FUNCTION OF ITS ARGUMENTS AND MUTATES NOTHING, because the sync
// command has to know what the tracker WOULD look like before it commits to it
// -- a command that mutated and then refused would leave the room holding half
// of what nobody asked for.
//
// THE SUCCESSOR IS CHOSEN FROM THE OLD ORDER, before the deletion, because "the
// next combatant" is a fact about the order the GM built.
func dropped(entries []InitiativeEntry, active *ulid.ULID, gone func(ulid.ULID) bool) ([]InitiativeEntry, *ulid.ULID, bool) {
	empties := func(e InitiativeEntry) bool {
		if len(e.PawnIDs) == 0 {
			return false
		}
		for _, id := range e.PawnIDs {
			if !gone(id) {
				return false
			}
		}

		return true
	}

	changed := false

	at := -1
	if active != nil {
		at = slices.IndexFunc(entries, func(e InitiativeEntry) bool { return e.ID == *active })
	}

	if at >= 0 && empties(entries[at]) {
		var successor *ulid.ULID
		n := len(entries)
		for i := 1; i < n; i++ {
			e := entries[(at+i)%n]
			if !empties(e) {
				id := e.ID
				successor = &id

				break
			}
		}
		active = successor
		changed = true
	}

	kept := make([]InitiativeEntry, 0, len(entries))
	for _, e := range entries {
		if empties(e) {
			changed = true

			continue
		}

		members := make([]ulid.ULID, 0, len(e.PawnIDs))
		for _, id := range e.PawnIDs {
			if gone(id) {
				changed = true

				continue
			}
			members = append(members, id)
		}
		e.PawnIDs = members

		kept = append(kept, e)
	}

	return kept, active, changed
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

	// ONE PAWN IS IN THE ORDER ONCE, across every line and not only within one.
	// A goblin in its group AND on a line of its own would take two turns and
	// tick its conditions twice, and the second of those is silent.
	claimed := map[ulid.ULID]bool{}

	for _, e := range c.Entries {
		if err := checkRequiredName("tracker entry", e.Name); err != nil {
			return nil, err
		}
		for _, id := range e.PawnIDs {
			if _, err := s.requirePawn(id); err != nil {
				return nil, err
			}
			if claimed[id] {
				return nil, invalid("Duplicate pawn", "The same pawn is in the initiative order twice.")
			}
			claimed[id] = true
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

	// ANY PAWN IN THE ACTIVE LINE, not the first one. A line is a list now, and
	// a player who owns one of the creatures acting on this count is whose turn
	// it is -- which for a solo line is the only member and is the rule this
	// has always had.
	if s.Initiative.Active != nil {
		if e := s.entry(*s.Initiative.Active); e != nil {
			for _, id := range e.PawnIDs {
				if p := s.Pawn(id); p != nil && p.OwnerID != nil && *p.OwnerID == a.ID {
					return nil
				}
			}
		}
	}

	return forbidden("Not your turn", "Only the GM, or whoever's turn it is, can advance the tracker.")
}

func (c *InitiativeNext) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	if len(s.Initiative.Entries) == 0 {
		return nil, invalid("Nothing to advance", "There is nothing in the initiative tracker.")
	}

	n := len(s.Initiative.Entries)

	from := -1
	if s.Initiative.Active != nil {
		from = slices.IndexFunc(s.Initiative.Entries, func(e InitiativeEntry) bool {
			return e.ID == *s.Initiative.Active
		})
	}

	var next int
	switch {
	case from < 0:
		// Nobody was acting, so nobody's turn just ended and only the
		// start-of-turn conditions tick. It still skips: a fight opened on a
		// corpse is a fight whose first turn is spent pressing the button
		// again.
		//
		// THE ROUND IS NOT SET HERE. A tracker with lines in it is already in
		// round one -- Normalize holds that -- and this branch is reached
		// again mid-fight whenever the acting line has gone, which is a
		// removal or a sync and not the fight starting over.
		next = 0
		for i := 0; i < n; i++ {
			if !s.skips(s.Initiative.Entries[i]) {
				next = i

				break
			}
		}

	default:
		// Walk forward until something is worth acting on. steps is at most n,
		// which lands back on the line we started from -- a whole lap of
		// corpses -- and the fallback below is what makes that a press rather
		// than a hang.
		steps := 1
		for ; steps <= n; steps++ {
			if !s.skips(s.Initiative.Entries[(from+steps)%n]) {
				break
			}
		}
		if steps > n {
			steps = 1
		}

		next = (from + steps) % n

		// THE ROUND COUNTS THE SEAM AND NOT THE SKIPS. Crossing the end of the
		// list increments it once, however many lines were passed over on the
		// way -- because a round is a lap of the table and skipping four dead
		// goblins on the way past is still one lap.
		if from+steps >= n {
			s.Initiative.Round++
		}
	}

	// CONDITIONS TICK ON THE TWO LINES AT THE SEAM, which is the 5e rule spelled
	// out: "until the end of your next turn" counts down as your turn ends, and
	// "until the start of your next turn" as it begins. A duration of -1 never
	// counts and never expires -- it is there until somebody takes it off.
	//
	// A GROUP TICKS EVERY MEMBER, because the group is one turn and every
	// creature in it took it.
	//
	// A LINE THAT WAS SKIPPED TICKS NOTHING. Its turn did not happen, and a
	// condition counting down on a corpse is bookkeeping about a creature that
	// has stopped taking turns.
	touched := map[ulid.ULID]bool{}
	if from >= 0 {
		s.tick(s.Initiative.Entries[from], ClearEnd, touched)
	}
	s.tick(s.Initiative.Entries[next], ClearStart, touched)

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

// skips is whether the turn passes over this line, and it is one clause plus
// the exception that clause exists for.
//
// A DEAD MONSTER'S TURN IS A TURN WASTED and the button should not spend one on
// it. A PLAYER AT ZERO IS MAKING DEATH SAVING THROWS -- three saves against
// three failures, the most consequential turn of that character's life -- and
// an app that skipped it would be an app that killed somebody's character by
// omission. That one clause is the whole difference between the two kinds of
// creature this app draws.
//
// A GROUP WITH ONE GOBLIN STILL STANDING IS NOT SKIPPED, which is what makes
// the count on its card matter. A LINE WITH NO PAWNS -- a lair action -- is
// never skipped: nothing about it can be dead.
//
// THE GM CAN STILL ACTIVATE A CORPSE by clicking its card. This is what the
// button does, not a rule about what may be active.
func (s *State) skips(e InitiativeEntry) bool {
	found := false

	for _, id := range e.PawnIDs {
		p := s.Pawn(id)
		if p == nil {
			continue
		}
		found = true

		if p.Kind == PawnPlayer || !Dead(*p) {
			return false
		}
	}

	return found
}

// tick counts one line's conditions down at one end of its turn and records
// every pawn it changed.
func (s *State) tick(e InitiativeEntry, when ClearTrigger, touched map[ulid.ULID]bool) {
	for _, id := range e.PawnIDs {
		p := s.Pawn(id)
		if p == nil {
			continue
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
			continue
		}

		p.Conditions = kept
		touched[p.ID] = true
	}
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

// InitiativeSync builds the tracker from what is on the table, and builds it
// again mid-fight to bring in reinforcements and take out the corpses. It is
// how a fight starts and how it grows, and it is the only gesture in this
// feature that has to look at the whole room.
//
// IT IS A COMMAND AND NOT A CONTROLLER, which is the one exception to
// initiative.set being the only editing command. Every clause below is a rule
// about room state that the room is the only thing holding: which floors have
// players on them, which pawns are visible, which are dead, what the grouping
// setting is, and what is already in the tracker. A controller would read all
// of that through the hub, decide, and dispatch a set -- and the window between
// the read and the dispatch is wider here than anywhere else, because the read
// is the whole table. This is a dozen lines beside the rules it depends on, and
// it is atomic on the room's own goroutine.
//
// THE FLOORS ARE THE ONES A PLAYER IS STANDING ON. A fight is where the party
// is; a monster waiting three floors up is not in this fight, and the GM's own
// view of another floor is not evidence about where anybody is.
//
// EVERYTHING ALREADY IN THE TRACKER STAYS WHERE IT IS. Sync never reorders. A
// GM who has dragged the order into shape and presses it again gets their order
// back with more on the end.
//
// AND IT ONLY EVER TAKES CORPSES OUT. A creature that has been hidden, or has
// walked off the party's floor, keeps its place -- it is still in the fight and
// the GM put it there; taking it out would also undo the Add to initiative on
// the pawn menu, whose whole purpose is to put something in that Sync would not
// have found.
type InitiativeSync struct{}

func (c *InitiativeSync) Authorize(s *State, a Actor) error {
	return requireGM(a, "sync the initiative tracker")
}

func (c *InitiativeSync) Apply(s *State, a Actor, env Env) ([]Emission, error) {
	// A DEAD MONSTER GOES AND A DEAD PLAYER PAWN STAYS, which is the same
	// exception the turn key makes and is written in both places because it is
	// the same fact about the same rule: a player at zero is making death
	// saving throws and is still in the fight.
	entries, active, _ := dropped(s.Initiative.Entries, s.Initiative.Active, func(id ulid.ULID) bool {
		p := s.Pawn(id)

		return p == nil || (p.Kind != PawnPlayer && Dead(*p))
	})

	floors := map[ulid.ULID]bool{}
	for _, p := range s.Pawns {
		if p.Kind == PawnPlayer && p.Visible {
			floors[p.LayerID] = true
		}
	}

	already := map[ulid.ULID]bool{}
	for _, e := range entries {
		for _, id := range e.PawnIDs {
			already[id] = true
		}
	}

	grouped := s.Table.InitiativeGrouping != GroupIndividual

	// WHERE A REINFORCEMENT GOES. Three more goblins arriving in round four are
	// more goblins, not a second goblin turn -- so a new monster whose key
	// matches a line already in the order joins it. The key is recomputed from
	// the members rather than stored, so there is nothing to keep in step.
	joins := map[string]int{}
	if grouped {
		for i, e := range entries {
			key := s.groupKey(e)
			if key == "" {
				continue
			}
			if _, seen := joins[key]; !seen {
				joins[key] = i
			}
		}
	}

	var added []InitiativeEntry
	opened := map[string]int{}

	take := func(p Pawn) {
		if grouped && p.Kind == PawnMonster {
			key := MonsterKey(p)
			if i, ok := joins[key]; ok {
				entries[i].PawnIDs = append(entries[i].PawnIDs, p.ID)

				return
			}
			if i, ok := opened[key]; ok {
				added[i].PawnIDs = append(added[i].PawnIDs, p.ID)

				return
			}
			opened[key] = len(added)
		}

		added = append(added, InitiativeEntry{
			ID:      env.id(),
			PawnIDs: []ulid.ULID{p.ID},
			Name:    p.Name,
		})
	}

	wanted := func(p Pawn) bool {
		return p.Visible && floors[p.LayerID] && !already[p.ID]
	}

	// THE PARTY GOES IN FIRST, in the order the table holds them, and the
	// monsters follow. Nothing about a fight says the party acts first -- the
	// GM drags -- but a fresh tracker has to start in SOME order, and one that
	// begins with the people who are going to be dragging is a better place to
	// start than one that interleaves by id.
	for _, p := range s.Pawns {
		if p.Kind == PawnPlayer && wanted(p) {
			take(p)
		}
	}
	for _, p := range s.Pawns {
		if p.Kind != PawnMonster && p.Kind != PawnNPC {
			continue
		}
		if wanted(p) && !Dead(p) {
			take(p)
		}
	}

	entries = append(entries, added...)

	if len(entries) > InitiativeMax {
		return nil, invalid("Too many entries", "The tracker holds at most 200 entries.")
	}
	if len(entries) == 0 {
		return nil, invalid("Nothing to sync",
			"There is nobody on a floor a player is standing on, so there is no order to build. Spawn the party first.")
	}

	if active != nil && !hasEntry(entries, *active) {
		active = nil
	}

	// THE ROUND IS NOT RESET, for InitiativeSet's reason: reinforcements
	// arriving in round four are not the fight starting again, and the round is
	// what the party's spell durations are counted in.
	s.Initiative.Entries = entries
	s.Initiative.Active = active
	s.Normalize()

	return initiativeUpdated(s), nil
}

func (s *State) groupKey(e InitiativeEntry) string { return GroupKey(e, s.Pawn) }

// GroupKey is the identity of the line a reinforcement would join, or empty for
// a line that is not a monster group at all.
//
// IT IS READ OFF THE FIRST MEMBER because every member of a group was put there
// by having the same key. A line whose first member has gone from the table
// answers empty, which is a reinforcement that starts a new line rather than
// one that joins a line nothing can be checked against.
//
// IT TAKES A LOOKUP RATHER THAN A STATE because the other caller is the HTTP
// handler behind Add to initiative, which holds a map of projected pawns and
// not the room. Exporting the rule is what keeps the two from disagreeing about
// what makes two goblins the same goblin.
func GroupKey(e InitiativeEntry, pawn func(ulid.ULID) *Pawn) string {
	for _, id := range e.PawnIDs {
		p := pawn(id)
		if p == nil {
			continue
		}
		if p.Kind != PawnMonster {
			return ""
		}

		return MonsterKey(*p)
	}

	return ""
}

// MonsterKey is what makes two monsters the same monster.
//
// IT IS THE MANUAL'S ID WHERE THERE IS ONE, AND THE NAME AND PICTURE WHERE
// THERE IS NOT. A pawn spawned from the manual carries MonsterID and that is
// the identity; a "Goblin" token dragged out of the asset library has none, and
// two of those are the same creature exactly when a table would say they are.
//
// THE PREFIX IS WHAT KEEPS THE TWO KINDS OF KEY APART, so that a monster whose
// name happens to read like a ULID cannot collide with one.
func MonsterKey(p Pawn) string {
	if p.MonsterID != nil {
		return "id:" + p.MonsterID.String()
	}

	return "name:" + p.Name + "\x00" + p.Image
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
