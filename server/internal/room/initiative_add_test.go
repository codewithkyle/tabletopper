package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)




func TestAddPutsAMonsterInItsGroup(t *testing.T) {
	manual := testID(900)
	goblin := func(w *world) ulid.ULID {
		return w.spawn(Pawn{Name: "Goblin", MonsterID: &manual, Visible: true})
	}

	t.Run("grouped: the second goblin joins the first one's line", func(t *testing.T) {
		w := newWorld(t)
		first, second := goblin(w), goblin(w)

		w.apply(&InitiativeAdd{Pawn: &first}, w.gm)
		w.apply(&InitiativeAdd{Pawn: &second}, w.gm)

		if got := w.s.Initiative.Entries; len(got) != 1 || len(got[0].PawnIDs) != 2 {
			t.Fatalf("the second goblin made %d lines holding %v", len(got), got)
		}
	})

	t.Run("a different monster is a different line", func(t *testing.T) {
		w := newWorld(t)
		first := goblin(w)
		ogre := w.spawn(Pawn{Name: "Ogre", Visible: true})

		w.apply(&InitiativeAdd{Pawn: &first}, w.gm)
		w.apply(&InitiativeAdd{Pawn: &ogre}, w.gm)

		if got := w.s.Initiative.Entries; len(got) != 2 || got[1].Name != "Ogre" {
			t.Fatalf("the ogre did not get a line of its own: %v", got)
		}
	})

	t.Run("individual is always a new line", func(t *testing.T) {
		w := newWorld(t)
		w.apply(&TableSetOptions{PawnLabels: LabelsDefault, PlayersCanDraw: true, InitiativeGrouping: GroupIndividual}, w.gm)
		first, second := goblin(w), goblin(w)

		w.apply(&InitiativeAdd{Pawn: &first}, w.gm)
		w.apply(&InitiativeAdd{Pawn: &second}, w.gm)

		if got := w.s.Initiative.Entries; len(got) != 2 {
			t.Fatalf("the second goblin was grouped in individual mode: %v", got)
		}
	})

	t.Run("a creature already in the order is refused", func(t *testing.T) {
		w := newWorld(t)
		first := goblin(w)

		w.apply(&InitiativeAdd{Pawn: &first}, w.gm)
		e := w.refuse(&InitiativeAdd{Pawn: &first}, w.gm, CodeInvalid)
		if e.Heading != "Already in the order" {
			t.Errorf("heading = %q, want the sentence the strip shows", e.Heading)
		}
	})

	t.Run("a name or a pawn and never both", func(t *testing.T) {
		w := newWorld(t)
		first := goblin(w)

		w.refuse(&InitiativeAdd{}, w.gm, CodeInvalid)
		w.refuse(&InitiativeAdd{Name: "Lair action", Pawn: &first}, w.gm, CodeInvalid)
		w.apply(&InitiativeAdd{Name: "Lair action"}, w.gm)
	})
}


func TestTheStripGesturesEditTheOrderInPlace(t *testing.T) {
	w := newWorld(t)
	w.apply(&InitiativeSet{Entries: []InitiativeEntry{
		{Name: "Ari"}, {Name: "Goblin"}, {Name: "Lair action"},
	}}, w.gm)
	ari, goblin, lair := w.s.Initiative.Entries[0].ID, w.s.Initiative.Entries[1].ID, w.s.Initiative.Entries[2].ID

	t.Run("activate gives the turn to a line and refuses one that is gone", func(t *testing.T) {
		w.apply(&InitiativeActivate{Entry: goblin}, w.gm)
		if got := w.s.Initiative.Active; got == nil || *got != goblin {
			t.Fatalf("active = %v, want the goblin", got)
		}
		w.refuse(&InitiativeActivate{Entry: testID(999)}, w.gm, CodeNotFound)
	})

	t.Run("reorder takes exactly the tracker's ids", func(t *testing.T) {
		w.apply(&InitiativeReorder{IDs: []ulid.ULID{lair, goblin, ari}}, w.gm)
		if got := w.s.Initiative.Entries; got[0].ID != lair || got[2].ID != ari {
			t.Fatalf("order = %v, want lair, goblin, ari", got)
		}
		if got := w.s.Initiative.Active; got == nil || *got != goblin {
			t.Errorf("a reorder moved the turn: %v", got)
		}

		w.refuse(&InitiativeReorder{IDs: []ulid.ULID{lair, goblin}}, w.gm, CodeInvalid)
		w.refuse(&InitiativeReorder{IDs: []ulid.ULID{lair, goblin, goblin}}, w.gm, CodeInvalid)
		w.refuse(&InitiativeReorder{IDs: []ulid.ULID{lair, goblin, testID(999)}}, w.gm, CodeInvalid)
	})

	t.Run("removing the acting line moves the turn on in the old order", func(t *testing.T) {
		
		
		w.apply(&InitiativeRemove{Entry: goblin}, w.gm)
		if got := w.s.Initiative.Active; got == nil || *got != ari {
			t.Fatalf("active = %v, want Ari", got)
		}
		if len(w.s.Initiative.Entries) != 2 {
			t.Fatalf("entries = %v, want two", w.s.Initiative.Entries)
		}

		w.refuse(&InitiativeRemove{Entry: goblin}, w.gm, CodeNotFound)
	})

	t.Run("removing the last line empties the tracker", func(t *testing.T) {
		w.apply(&InitiativeRemove{Entry: lair}, w.gm)
		w.apply(&InitiativeRemove{Entry: ari}, w.gm)
		if len(w.s.Initiative.Entries) != 0 || w.s.Initiative.Active != nil || w.s.Initiative.Round != 0 {
			t.Fatalf("tracker = %+v, want empty and in no round", w.s.Initiative)
		}
	})
}
