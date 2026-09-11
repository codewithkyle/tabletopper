package room

import (
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

func Reduce(s *State, ev Event) error {
	if ev == nil || ev.Transient() {
		return nil
	}
	switch e := ev.(type) {
	case *Snapshot:
		*s = e.State.Clone()
		return nil
	case *RoomUpdated:
		s.Room = e.Room
	case *TableUpdated:
		s.Table = CloneTable(e.Table)
	case *InitiativeUpdated:
		s.Initiative = cloneInitiative(e.Initiative)
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
	case *FogCleared:
		s.Fog = slices.DeleteFunc(s.Fog, func(f FogShape) bool { return f.LayerID == e.Layer })
	case *StrokeCleared:
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return st.LayerID == e.Layer })
	case *StrokeErased:
		s.Strokes = slices.DeleteFunc(s.Strokes, func(st Stroke) bool { return slices.Contains(e.IDs, st.ID) })
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
		return fmt.Errorf("room: no reduction for %s", ev.eventType())
	}
	s.Normalize()
	return nil
}
func upsert[T any](into *[]T, v T, id func(T) ulid.ULID) {
	for i := range *into {
		if id((*into)[i]) == id(v) {
			(*into)[i] = v
			return
		}
	}
	*into = append(*into, v)
}
