package room

import (
	"reflect"
	"slices"

	"github.com/oklog/ulid/v2"
)

func Derive(before, after *State, role Role) []Change {
	b, a := before.Project(role), after.Project(role)
	var out []Change
	if !reflect.DeepEqual(b.Room, a.Room) {
		out = append(out, &RoomUpdated{Room: a.Room})
	}
	out = append(out, playerDiff.derive(b.Players, a.Players)...)
	out = append(out, fogDiff.derive(b.Fog, a.Fog)...)
	out = append(out, strokeDiff.derive(b.Strokes, a.Strokes)...)
	out = append(out, pawnDiff.derive(b.Pawns, a.Pawns)...)
	out = append(out, rollDiff.derive(b.Rolls, a.Rolls)...)
	if !reflect.DeepEqual(b.Music, a.Music) {
		out = append(out, &MusicUpdated{Music: CloneMusic(a.Music)})
	}
	if !reflect.DeepEqual(b.Initiative, a.Initiative) {
		out = append(out, &InitiativeUpdated{Initiative: cloneInitiative(a.Initiative)})
	}
	if !reflect.DeepEqual(b.Table.Layers, a.Table.Layers) {
		out = append(out, &LayersUpdated{Layers: cloneLayers(a.Table.Layers)})
	}
	if b.Table.TableSettings != a.Table.TableSettings {
		out = append(out, &TableUpdated{Table: a.Table.TableSettings})
	}
	return out
}

var playerDiff = diff[Player]{
	id:       func(p Player) ulid.ULID { return p.ID },
	clone:    clonePlayer,
	items:    func(s *State) *[]Player { return &s.Players },
	upserted: func(items []Player) Change { return &PlayersUpserted{Players: items} },
	removed:  func(ids []ulid.ULID) Change { return &PlayersRemoved{IDs: ids} },
}
var pawnDiff = diff[Pawn]{
	id:       func(p Pawn) ulid.ULID { return p.ID },
	clone:    clonePawn,
	items:    func(s *State) *[]Pawn { return &s.Pawns },
	upserted: func(items []Pawn) Change { return &PawnsUpserted{Pawns: items} },
	removed:  func(ids []ulid.ULID) Change { return &PawnsRemoved{IDs: ids} },
	delta:    movedPawns,
}
var fogDiff = diff[FogShape]{
	id:       func(f FogShape) ulid.ULID { return f.ID },
	clone:    cloneShape,
	items:    func(s *State) *[]FogShape { return &s.Fog },
	upserted: func(items []FogShape) Change { return &FogUpserted{Shapes: items} },
	removed:  func(ids []ulid.ULID) Change { return &FogRemoved{IDs: ids} },
}
var rollDiff = diff[Roll]{
	id:       func(r Roll) ulid.ULID { return r.ID },
	clone:    cloneRoll,
	items:    func(s *State) *[]Roll { return &s.Rolls },
	upserted: func(items []Roll) Change { return &RollsUpserted{Rolls: items} },
	removed:  func(ids []ulid.ULID) Change { return &RollsRemoved{IDs: ids} },
}
var strokeDiff = diff[Stroke]{
	id:       func(st Stroke) ulid.ULID { return st.ID },
	clone:    cloneStroke,
	items:    func(s *State) *[]Stroke { return &s.Strokes },
	upserted: func(items []Stroke) Change { return &StrokesUpserted{Strokes: items} },
	removed:  func(ids []ulid.ULID) Change { return &StrokesRemoved{IDs: ids} },
	delta:    strokeDeltas,
}

type diff[T any] struct {
	id       func(T) ulid.ULID
	clone    func(T) T
	items    func(*State) *[]T
	upserted func([]T) Change
	removed  func([]ulid.ULID) Change
	delta    func([]pair[T]) ([]Change, []pair[T])
}

func (d diff[T]) derive(before, after []T) []Change {
	gone, added, changed := compare(before, after, d.id)
	var out []Change
	if len(gone) > 0 {
		ids := make([]ulid.ULID, 0, len(gone))
		for _, v := range gone {
			ids = append(ids, d.id(v))
		}
		out = append(out, d.removed(ids))
	}
	var deltas []Change
	if d.delta != nil {
		deltas, changed = d.delta(changed)
	}
	up := make([]T, 0, len(added)+len(changed))
	for _, v := range added {
		up = append(up, d.clone(v))
	}
	for _, p := range changed {
		up = append(up, d.clone(p.now))
	}
	if len(up) > 0 {
		out = append(out, d.upserted(up))
	}
	return append(out, deltas...)
}
func movedPawns(changed []pair[Pawn]) ([]Change, []pair[Pawn]) {
	at := make([]PawnPosition, 0, len(changed))
	for _, p := range changed {
		still := p.was
		still.X, still.Y = p.now.X, p.now.Y
		if !reflect.DeepEqual(still, p.now) {
			return nil, changed
		}
		at = append(at, PawnPosition{ID: p.now.ID, X: p.now.X, Y: p.now.Y})
	}
	if len(at) == 0 {
		return nil, changed
	}
	return []Change{&PawnsMoved{Pawns: at}}, nil
}
func strokeDeltas(changed []pair[Stroke]) ([]Change, []pair[Stroke]) {
	var out []Change
	var rest []pair[Stroke]
	for _, p := range changed {
		if ch := strokeDelta(p.was, p.now); ch != nil {
			out = append(out, ch)
			continue
		}
		rest = append(rest, p)
	}
	return out, rest
}
func strokeDelta(was, now Stroke) Change {
	if !reflect.DeepEqual(strokeBody(was), strokeBody(now)) {
		return nil
	}
	if was.Done == now.Done && prefixes(was.Points, now.Points) {
		return &StrokeExtended{ID: now.ID, Points: cloneSlice(now.Points[len(was.Points):])}
	}
	if now.Done && !was.Done && slices.Equal(was.Points, now.Points) {
		return &StrokeEnded{ID: now.ID}
	}
	return nil
}
func strokeBody(st Stroke) Stroke {
	st.Points, st.Done = nil, false
	return st
}
func prefixes(head, whole []int) bool {
	return len(head) < len(whole) && slices.Equal(head, whole[:len(head)])
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
