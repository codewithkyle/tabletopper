package room

import (
	"fmt"
	"slices"

	"github.com/oklog/ulid/v2"
)

func Reduce(s *State, ch Change) error {
	entry, ok := changeTypes[ch.changeType()]
	if !ok {
		return fmt.Errorf("room: no reduction for %s", ch.changeType())
	}
	entry.reduce(s, ch)
	s.Normalize()
	return nil
}

type reduction struct {
	build  func() Change
	reduce func(*State, Change)
}

var changeTypes = map[string]reduction{
	"room.updated": {
		build:  func() Change { return &RoomUpdated{} },
		reduce: func(s *State, ch Change) { s.Room = ch.(*RoomUpdated).Room },
	},
	"table.updated": {
		build:  func() Change { return &TableUpdated{} },
		reduce: func(s *State, ch Change) { s.Table.TableSettings = ch.(*TableUpdated).Table },
	},
	"layers.updated": {
		build:  func() Change { return &LayersUpdated{} },
		reduce: func(s *State, ch Change) { s.Table.Layers = cloneLayers(ch.(*LayersUpdated).Layers) },
	},
	"music.updated": {
		build:  func() Change { return &MusicUpdated{} },
		reduce: func(s *State, ch Change) { s.Music = CloneMusic(ch.(*MusicUpdated).Music) },
	},
	"initiative.updated": {
		build:  func() Change { return &InitiativeUpdated{} },
		reduce: func(s *State, ch Change) { s.Initiative = cloneInitiative(ch.(*InitiativeUpdated).Initiative) },
	},
	"players.upserted": {
		build:  func() Change { return &PlayersUpserted{} },
		reduce: upserts(playerDiff, func(ch Change) []Player { return ch.(*PlayersUpserted).Players }),
	},
	"players.removed": {
		build:  func() Change { return &PlayersRemoved{} },
		reduce: removes(playerDiff, func(ch Change) []ulid.ULID { return ch.(*PlayersRemoved).IDs }),
	},
	"pawns.upserted": {
		build:  func() Change { return &PawnsUpserted{} },
		reduce: upserts(pawnDiff, func(ch Change) []Pawn { return ch.(*PawnsUpserted).Pawns }),
	},
	"pawns.removed": {
		build:  func() Change { return &PawnsRemoved{} },
		reduce: removes(pawnDiff, func(ch Change) []ulid.ULID { return ch.(*PawnsRemoved).IDs }),
	},
	"pawns.moved": {
		build: func() Change { return &PawnsMoved{} },
		reduce: func(s *State, ch Change) {
			for _, at := range ch.(*PawnsMoved).Pawns {
				if p := s.Pawn(at.ID); p != nil {
					p.X, p.Y = at.X, at.Y
				}
			}
		},
	},
	"fog.upserted": {
		build:  func() Change { return &FogUpserted{} },
		reduce: upserts(fogDiff, func(ch Change) []FogShape { return ch.(*FogUpserted).Shapes }),
	},
	"fog.removed": {
		build:  func() Change { return &FogRemoved{} },
		reduce: removes(fogDiff, func(ch Change) []ulid.ULID { return ch.(*FogRemoved).IDs }),
	},
	"rolls.upserted": {
		build:  func() Change { return &RollsUpserted{} },
		reduce: upserts(rollDiff, func(ch Change) []Roll { return ch.(*RollsUpserted).Rolls }),
	},
	"rolls.removed": {
		build:  func() Change { return &RollsRemoved{} },
		reduce: removes(rollDiff, func(ch Change) []ulid.ULID { return ch.(*RollsRemoved).IDs }),
	},
	"strokes.upserted": {
		build:  func() Change { return &StrokesUpserted{} },
		reduce: upserts(strokeDiff, func(ch Change) []Stroke { return ch.(*StrokesUpserted).Strokes }),
	},
	"strokes.removed": {
		build:  func() Change { return &StrokesRemoved{} },
		reduce: removes(strokeDiff, func(ch Change) []ulid.ULID { return ch.(*StrokesRemoved).IDs }),
	},
	"strokes.extended": {
		build: func() Change { return &StrokeExtended{} },
		reduce: func(s *State, ch Change) {
			e := ch.(*StrokeExtended)
			if st := s.Stroke(e.ID); st != nil {
				st.Points = append(st.Points, e.Points...)
			}
		},
	},
	"strokes.ended": {
		build: func() Change { return &StrokeEnded{} },
		reduce: func(s *State, ch Change) {
			if st := s.Stroke(ch.(*StrokeEnded).ID); st != nil {
				st.Done = true
			}
		},
	},
}

func upserts[T any](d diff[T], from func(Change) []T) func(*State, Change) {
	return func(s *State, ch Change) {
		into := d.items(s)
		for _, v := range from(ch) {
			upsert(into, d.clone(v), d.id)
		}
	}
}
func removes[T any](d diff[T], from func(Change) []ulid.ULID) func(*State, Change) {
	return func(s *State, ch Change) {
		gone := from(ch)
		into := d.items(s)
		*into = slices.DeleteFunc(*into, func(v T) bool { return slices.Contains(gone, d.id(v)) })
	}
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
func ChangePrototypes() map[string]Change {
	out := make(map[string]Change, len(changeTypes))
	for name, entry := range changeTypes {
		out[name] = entry.build()
	}
	return out
}
