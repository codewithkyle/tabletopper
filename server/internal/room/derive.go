package room

import (
	"reflect"
	"slices"

	"github.com/oklog/ulid/v2"
)

func Derive(before, after *State, role Role) []Event {
	b, a := before.Project(role), after.Project(role)
	var out []Event
	if !reflect.DeepEqual(b.Room, a.Room) {
		out = append(out, &RoomUpdated{Room: a.Room})
	}
	out = append(out, derivePlayers(b.Players, a.Players)...)
	out = append(out, deriveFog(b.Fog, a.Fog)...)
	out = append(out, deriveStrokes(b.Strokes, a.Strokes)...)
	out = append(out, derivePawns(b.Pawns, a.Pawns)...)
	if !reflect.DeepEqual(b.Initiative, a.Initiative) {
		out = append(out, &InitiativeUpdated{Initiative: cloneInitiative(a.Initiative)})
	}
	if !reflect.DeepEqual(b.Table, a.Table) {
		out = append(out, &TableUpdated{Table: CloneTable(a.Table)})
	}
	return out
}
func derivePlayers(before, after []Player) []Event {
	gone, added, changed := compare(before, after, func(p Player) ulid.ULID { return p.ID })
	out := make([]Event, 0, len(gone)+len(added)+len(changed))
	for _, p := range gone {
		out = append(out, &PlayerLeft{ID: p.ID})
	}
	for _, p := range added {
		out = append(out, &PlayerJoined{Player: clonePlayer(p)})
	}
	for _, pair := range changed {
		out = append(out, &PlayerUpdated{Player: clonePlayer(pair.now)})
	}
	return out
}
func deriveFog(before, after []FogShape) []Event {
	gone, added, changed := compare(before, after, func(f FogShape) ulid.ULID { return f.ID })
	out := make([]Event, 0, len(gone)+len(added)+len(changed))
	for _, f := range gone {
		out = append(out, &FogRemoved{ID: f.ID})
	}
	for _, f := range added {
		out = append(out, &FogAdded{Shape: cloneShape(f)})
	}
	for _, pair := range changed {
		out = append(out, &FogAdded{Shape: cloneShape(pair.now)})
	}
	return out
}
func deriveStrokes(before, after []Stroke) []Event {
	gone, added, changed := compare(before, after, func(st Stroke) ulid.ULID { return st.ID })
	out := make([]Event, 0, len(added)+len(changed)+1)
	if len(gone) > 0 {
		ids := make([]ulid.ULID, 0, len(gone))
		for _, st := range gone {
			ids = append(ids, st.ID)
		}
		out = append(out, &StrokeErased{IDs: ids})
	}
	for _, st := range added {
		out = append(out, &StrokeBegan{Stroke: cloneStroke(st)})
	}
	for _, pair := range changed {
		out = append(out, strokeChange(pair.was, pair.now))
	}
	return out
}
func strokeChange(was, now Stroke) Event {
	if !reflect.DeepEqual(strokeBody(was), strokeBody(now)) {
		return &StrokeBegan{Stroke: cloneStroke(now)}
	}
	if was.Done == now.Done && prefixes(was.Points, now.Points) {
		return &StrokeExtended{ID: now.ID, Points: cloneSlice(now.Points[len(was.Points):])}
	}
	if now.Done && !was.Done && slices.Equal(was.Points, now.Points) {
		return &StrokeEnded{ID: now.ID}
	}
	return &StrokeBegan{Stroke: cloneStroke(now)}
}
func strokeBody(st Stroke) Stroke {
	st.Points, st.Done = nil, false
	return st
}
func prefixes(head, whole []int) bool {
	return len(head) < len(whole) && slices.Equal(head, whole[:len(head)])
}
func derivePawns(before, after []Pawn) []Event {
	gone, added, changed := compare(before, after, func(p Pawn) ulid.ULID { return p.ID })
	out := make([]Event, 0, len(gone)+len(added)+len(changed))
	for _, p := range gone {
		out = append(out, &PawnRemoved{ID: p.ID})
	}
	for _, p := range added {
		out = append(out, &PawnSpawned{Pawn: clonePawn(p)})
	}
	if at := onlyMoved(changed); at != nil {
		return append(out, &PawnMoved{Pawns: at})
	}
	for _, pair := range changed {
		out = append(out, &PawnUpdated{Pawn: clonePawn(pair.now)})
	}
	return out
}
func onlyMoved(changed []pair[Pawn]) []PawnPosition {
	at := make([]PawnPosition, 0, len(changed))
	for _, p := range changed {
		still := p.was
		still.X, still.Y = p.now.X, p.now.Y
		if !reflect.DeepEqual(still, p.now) {
			return nil
		}
		at = append(at, PawnPosition{ID: p.now.ID, X: p.now.X, Y: p.now.Y})
	}
	if len(at) == 0 {
		return nil
	}
	return at
}

type pair[T any] struct {
	was T
	now T
}

func compare[T any](before, after []T, id func(T) ulid.ULID) (gone, added []T, changed []pair[T]) {
	was := make(map[ulid.ULID]T, len(before))
	for _, v := range before {
		was[id(v)] = v
	}
	now := make(map[ulid.ULID]bool, len(after))
	for _, v := range after {
		now[id(v)] = true
	}
	for _, v := range before {
		if !now[id(v)] {
			gone = append(gone, v)
		}
	}
	for _, v := range after {
		old, had := was[id(v)]
		switch {
		case !had:
			added = append(added, v)
		case !reflect.DeepEqual(old, v):
			changed = append(changed, pair[T]{was: old, now: v})
		}
	}
	return gone, added, changed
}
