package room

import (
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

// Reduce applies one event to one state. It is the reference implementation of
// the client's reducer, and it exists in Go for two reasons that are really the
// same reason.
//
// FIRST, IT IS THE SPECIFICATION PHASE 3'S TYPESCRIPT IS PORTED FROM. Written
// out in one switch, the three sizing rules are visible as three shapes:
// singletons assign, collection items upsert or delete by id, and the three hot
// paths mutate named fields of an entity that is already there.
//
// SECOND, IT IS HOW THE CONVERGENCE PROPERTY BECOMES A TEST. Applying a
// command's emitted events to the projected before-state must produce the
// projected after-state, for both audiences, or the client's copy of the room
// drifts from the server's. That property is the whole justification for
// full-entity events, and without a reducer here it would be a claim rather
// than something that fails a build.
//
// IT DOES NOT TOUCH Seq. Tracking the sequence is the connection's job -- it is
// what notices a gap and asks for a resync -- and it is not part of the state a
// snapshot restores. Keeping it out of here is also what makes the fixtures
// comparable: the two audiences receive different numbers of events, so their
// sequence counters diverge by design and a reduced state that carried one
// could never equal a projected state.
//
// IT IS WRITTEN FOR CLARITY, NOT SPEED. It runs in tests and in the fixture
// generator and nowhere else; the server never reduces its own events, because
// the server is where they came from.
func Reduce(s *State, ev Event) error {
	if ev == nil || ev.Transient() {
		return nil
	}

	switch e := ev.(type) {

	// The snapshot is not a reduction, it is a replacement. Everything a client
	// held is discarded, which is exactly what a resync is for.
	case *Snapshot:
		*s = e.State.Clone()

		return nil

	// Singletons: assign the whole object.
	case *RoomUpdated:
		s.Room = e.Room
	case *TableUpdated:
		s.Table = cloneTable(e.Table)
	case *InitiativeUpdated:
		s.Initiative = cloneInitiative(e.Initiative)

	// Collection items: upsert or delete by id. Every one of these carries the
	// entity whole, so the upsert is one assignment and there is no question of
	// which fields the event meant to change.
	case *PlayerJoined:
		upsert(&s.Players, clonePlayer(e.Player), func(p Player) ulid.ULID { return p.ID })
	case *PlayerUpdated:
		upsert(&s.Players, clonePlayer(e.Player), func(p Player) ulid.ULID { return p.ID })
	case *PlayerLeft:
		s.Players = slices.DeleteFunc(s.Players, func(p Player) bool { return p.ID == e.ID })
	case *PawnSpawned:
		upsert(&s.Pawns, clonePawn(e.Pawn), func(p Pawn) ulid.ULID { return p.ID })
	case *PawnUpdated:
		upsert(&s.Pawns, clonePawn(e.Pawn), func(p Pawn) ulid.ULID { return p.ID })
	case *PawnRemoved:
		s.Pawns = slices.DeleteFunc(s.Pawns, func(p Pawn) bool { return p.ID == e.ID })
	case *FogAdded:
		upsert(&s.Fog, cloneShape(e.Shape), func(f FogShape) ulid.ULID { return f.ID })
	case *FogRemoved:
		s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.ID == e.ID })
	case *StrokeBegan:
		upsert(&s.Strokes, cloneStroke(e.Stroke), func(st Stroke) ulid.ULID { return st.ID })
	case *StrokeEnded:
		if st := s.Stroke(e.ID); st != nil {
			st.Done = true
		}

	// Collections cleared by layer, which is a delete that names a predicate
	// rather than a list of ids.
	case *FogCleared:
		s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == e.Layer })
	case *StrokeCleared:
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == e.Layer })
	case *StrokeErased:
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return slices.Contains(e.IDs, st.ID) })

	// The hot paths. An id that is not there is ignored rather than an error: a
	// player's copy of a move can legitimately name pawns they cannot see,
	// because the filtering happens per audience and a race can put an event
	// and a removal in either order.
	case *PawnMoved:
		for _, at := range e.Pawns {
			if p := s.Pawn(at.ID); p != nil {
				p.X, p.Y = at.X, at.Y
			}
		}
	case *StrokeExtended:
		if st := s.Stroke(e.ID); st != nil {
			st.Points = append(st.Points, e.Points...)
		}

	default:
		// Every event in the registry is above. Reaching here means one was
		// added without deciding what it does to the state, which is the bug
		// this branch exists to name.
		return fmt.Errorf("room: no reduction for %s", ev.eventType())
	}

	s.Normalize()

	return nil
}

// upsert replaces an entity by id or appends it. Fog keeps insertion order and
// the other three are sorted by Normalize, so appending is correct for all four.
func upsert[T any](into *[]T, v T, id func(T) ulid.ULID) {
	for i := range *into {
		if id((*into)[i]) == id(v) {
			(*into)[i] = v

			return
		}
	}

	*into = append(*into, v)
}
