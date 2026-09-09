package room

import (
	"testing"
)

// THE PROJECTION IS THE SECURITY BOUNDARY, so the tests for it are about what
// is absent from a player's copy rather than about what is present. A hidden
// pawn that arrived with a flag set would pass any test that only checked what
// the client draws.
func TestAPlayerNeverReceivesAHiddenPawn(t *testing.T) {
	w := newWorld(t)

	seen := w.spawn(Pawn{Name: "Goblin", Visible: true})
	hidden := w.spawn(Pawn{Name: "Ambusher", Visible: false})

	gm := w.s.Project(RoleGM)
	if len(gm.Pawns) != 2 {
		t.Fatalf("the GM's copy holds %d pawns, want both", len(gm.Pawns))
	}

	players := w.s.Project(RolePlayer)
	if len(players.Pawns) != 1 || players.Pawns[0].ID != seen {
		t.Fatalf("the players' copy holds %d pawns, want only the visible one", len(players.Pawns))
	}

	// Not "present with a flag": absent.
	for _, p := range players.Pawns {
		if p.ID == hidden {
			t.Fatal("the hidden pawn is in the players' copy")
		}
	}
}

// A pawn on another floor is as absent as a hidden one, and for the same
// reason: it is not on the table the player is looking at.
func TestAPlayerNeverReceivesAPawnFromAnotherLayer(t *testing.T) {
	w := newWorld(t)

	cellar := w.addLayer("Cellar")
	w.spawn(Pawn{Name: "Upstairs", Visible: true})
	w.spawn(Pawn{Name: "Downstairs", LayerID: cellar, Visible: true})

	players := w.s.Project(RolePlayer)
	if len(players.Pawns) != 1 || players.Pawns[0].Name != "Upstairs" {
		t.Fatalf("the players' copy holds %d pawns, want only the one on the active layer", len(players.Pawns))
	}
}

// THE THREE LABEL SETTINGS, against the four kinds of pawn they treat
// differently. A player's own character is never hidden from the table, and an
// object's hit points are the thing the party is currently hitting.
//
// THE HIT POINTS SURVIVE ALL THREE NOW, which is the reversal this test was
// rewritten for. What the setting decides is what an interface PRINTS, and that
// decision is made by ExactHP where the printing happens -- pawnView in
// internal/controllers/room-pawns.go and js/room/overlay.ts. What is still
// withheld here is armour class, which nothing on the table is drawn from, and
// the band, which is the instruction to print a word instead of a number.
func TestMonsterStatisticsProjectByTheRoomsSetting(t *testing.T) {
	kinds := []PawnKind{PawnPlayer, PawnMonster, PawnNPC, PawnObject}

	tests := []struct {
		setting PawnLabels
		banded  bool
		exact   bool
	}{
		{LabelsFull, false, true},
		{LabelsDefault, true, false},
		{LabelsNone, false, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.setting), func(t *testing.T) {
			w := newWorld(t)
			w.apply(&TableSetOptions{PawnLabels: tc.setting, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, w.gm)

			for _, kind := range kinds {
				p := Pawn{Kind: kind, Name: string(kind), Visible: true, HP: intp(5), MaxHP: intp(20), AC: intp(15)}
				if kind == PawnObject {
					p.Width, p.Height = DefaultCellSize, DefaultCellSize
				}
				w.spawn(p)
			}

			for _, p := range w.s.Project(RolePlayer).Pawns {
				monster := p.Kind == PawnMonster || p.Kind == PawnNPC

				// EVERY VIEWER IS SENT THE NUMBERS, under every setting. The
				// blood on a creature, the blood under it, the colour draining
				// out of it and its heartbeat are all drawn from them, and none
				// of that can be drawn out of a word.
				if p.HP == nil || *p.HP != 5 || p.MaxHP == nil || *p.MaxHP != 20 {
					t.Fatalf("a %s pawn lost its hit points under %q", p.Kind, tc.setting)
				}

				banded := monster && tc.banded
				if banded && p.HPBand == nil {
					t.Fatalf("a %s pawn has no band under %q", p.Kind, tc.setting)
				}
				if !banded && p.HPBand != nil {
					t.Fatalf("a %s pawn has a band under %q", p.Kind, tc.setting)
				}

				// ARMOUR CLASS IS THE LAST THING STILL WITHHELD. Telling the
				// party what to roll against is the same fight-solving
				// arithmetic the words exist to avoid, and unlike hit points
				// there is nothing on the table drawn from it -- so it survives
				// only on full, where everything does.
				if monster && !tc.exact && p.AC != nil {
					t.Fatalf("a %s pawn kept its armour class under %q", p.Kind, tc.setting)
				}
				if (!monster || tc.exact) && (p.AC == nil || *p.AC != 15) {
					t.Fatalf("a %s pawn lost its armour class under %q", p.Kind, tc.setting)
				}
			}

			// The GM's copy never carries a band, whatever the setting: a band
			// is a thing that exists only in a projection.
			for _, p := range w.s.Project(RoleGM).Pawns {
				if p.HPBand != nil {
					t.Fatalf("the GM's copy of a %s pawn carries a band", p.Kind)
				}
				if p.AC == nil || p.HP == nil {
					t.Fatalf("the GM's copy of a %s pawn was projected under %q", p.Kind, tc.setting)
				}
			}
		})
	}
}

// ExactHP IS THE SHOWING RULE and the only thing the label setting still
// decides. By the time it is asked, the numbers are on the pawn either way.
func TestExactHitPointsAreShownToTheRightViewers(t *testing.T) {
	settings := []PawnLabels{LabelsNone, LabelsDefault, LabelsFull}

	// A GM READS EVERYTHING, ALWAYS. There is no setting that hides a monster's
	// hit points from the person running it.
	for _, labels := range settings {
		for _, kind := range []PawnKind{PawnPlayer, PawnMonster, PawnNPC, PawnObject} {
			if !ExactHP(kind, labels, RoleGM) {
				t.Fatalf("the GM was refused a %s pawn's hit points under %q", kind, labels)
			}
		}
	}

	// A player's own character and an object are exact under every setting: a
	// character sheet is not a secret from the table, and a door's hit points
	// are the thing the party is currently hitting.
	for _, labels := range settings {
		for _, kind := range []PawnKind{PawnPlayer, PawnObject} {
			if !ExactHP(kind, labels, RolePlayer) {
				t.Fatalf("a player was refused a %s pawn's hit points under %q", kind, labels)
			}
		}
	}

	// A monster is the one case the setting touches, and only full shows it.
	for _, kind := range []PawnKind{PawnMonster, PawnNPC} {
		if ExactHP(kind, LabelsNone, RolePlayer) || ExactHP(kind, LabelsDefault, RolePlayer) {
			t.Fatalf("a player was shown a %s's hit points in a room that labels words", kind)
		}
		if !ExactHP(kind, LabelsFull, RolePlayer) {
			t.Fatalf("a player was refused a %s's hit points in an open room", kind)
		}
	}
}

// The band boundaries are three quarters, a half, a quarter and a twentieth,
// and they are compared by multiplication so that an awkward maximum has exact
// boundaries rather than ones that depend on which way integer division fell.
func TestTheHealthBandsSitWhereTheyAreDescribed(t *testing.T) {
	tests := []struct {
		hp, maxHP int
		want      HPBand
	}{
		// A maximum of 100, where every boundary is a whole number and every
		// one of them is ON the lower band: three quarters of a hundred is
		// bruised, not healthy.
		{100, 100, BandHealthy},
		{76, 100, BandHealthy},
		{75, 100, BandBruised},
		{51, 100, BandBruised},
		{50, 100, BandBloody},
		{26, 100, BandBloody},
		{25, 100, BandVeryBloody},
		{6, 100, BandVeryBloody},
		{5, 100, BandNearDeath},
		{1, 100, BandNearDeath},
		{0, 100, BandDead},

		// A maximum of 20, where a twentieth is one hit point: a goblin is
		// near death at exactly 1 and very bloody at 2.
		{20, 20, BandHealthy},
		{16, 20, BandHealthy},
		{15, 20, BandBruised},
		{10, 20, BandBloody},
		{5, 20, BandVeryBloody},
		{2, 20, BandVeryBloody},
		{1, 20, BandNearDeath},
		{0, 20, BandDead},

		// A maximum of 7: three quarters is 5.25, a half is 3.5 and a quarter
		// is 1.75, so every boundary falls between whole numbers.
		{7, 7, BandHealthy},
		{6, 7, BandHealthy},
		{5, 7, BandBruised},
		{4, 7, BandBruised},
		{3, 7, BandBloody},
		{2, 7, BandBloody},
		{1, 7, BandVeryBloody},

		// A creature already below the last cut is still not dead until it is
		// at zero, which is the whole reason the two words are separate.
		{1, 1000, BandNearDeath},
		{0, 1000, BandDead},

		// Overhealed past its own maximum, which a temporary hit point pool or
		// a GM raising the maximum after the fact both produce.
		{30, 20, BandHealthy},
	}

	for _, tc := range tests {
		got := hpBand(intp(tc.hp), intp(tc.maxHP))
		if got == nil {
			t.Fatalf("%d of %d hit points produced no band, want %s", tc.hp, tc.maxHP, tc.want)
		}
		if *got != tc.want {
			t.Fatalf("%d of %d hit points is %s, want %s", tc.hp, tc.maxHP, *got, tc.want)
		}
	}

	if hpBand(nil, intp(10)) != nil {
		t.Fatal("a pawn with no hit points was given a band")
	}
	if hpBand(intp(3), nil) != nil {
		t.Fatal("a pawn with no maximum was given a band")
	}
}

// CHANGING THE SETTING CHANGES EVERY MONSTER ON THE PLAYERS' SCREENS, and no
// pawn changed. Nothing else in the protocol would tell them, so the setting
// emits the pawns itself.
func TestChangingTheHitPointSettingReprojectsTheMonsters(t *testing.T) {
	w := newWorld(t)
	cellar := w.addLayer("Cellar")

	w.spawn(Pawn{Kind: PawnMonster, Name: "Goblin", Visible: true, HP: intp(5), MaxHP: intp(7)})
	w.spawn(Pawn{Kind: PawnNPC, Name: "Innkeeper", Visible: true, HP: intp(9), MaxHP: intp(9)})
	w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, HP: intp(12), MaxHP: intp(12)})
	w.spawn(Pawn{Kind: PawnMonster, Name: "Ambusher", Visible: false, HP: intp(5), MaxHP: intp(5)})
	w.spawn(Pawn{Kind: PawnMonster, Name: "Downstairs", LayerID: cellar, Visible: true, HP: intp(5), MaxHP: intp(5)})

	ems := w.apply(&TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: true, InitiativeGrouping: GroupMonsters}, w.gm)

	// The table, then one pawn per monster or npc the players can actually
	// see. Not the player's own pawn, whose projection did not change; not the
	// hidden one or the one downstairs, which are not in a player's state at
	// all and would be inserted by a pawn.updated naming them.
	equalStrings(t, "emissions", summary(ems), []string{
		"table.updated to all",
		"pawn.updated to players",
		"pawn.updated to players",
	})

	for _, em := range ems[1:] {
		p := em.Event.(*PawnUpdated).Pawn
		if p.HP == nil {
			t.Fatalf("%s was re-emitted without the exact hit points the change was about", p.Name)
		}
	}

	// Setting it to what it already is emits the table and nothing else.
	again := w.apply(&TableSetOptions{PawnLabels: LabelsFull, PlayersCanDraw: false, InitiativeGrouping: GroupMonsters}, w.gm)
	equalStrings(t, "emissions", summary(again), []string{"table.updated to all"})
}

// A kick tells the person leaving why, and tells everybody else that they are
// gone. Their pawns stay, because a character standing in the middle of a fight
// is part of the board.
func TestAKickTellsTheTargetAndTheRoomDifferentThings(t *testing.T) {
	w := newWorld(t)

	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: &testPlayerID})

	ems := w.apply(&PlayerKick{ID: testPlayerID}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{
		"player.kicked to player",
		"player.left to all",
	})

	if got := eventTypesOf(delivered(ems, w.gm, w.pc)); len(got) != 2 || got[0] != "player.kicked" {
		t.Fatalf("the kicked player received %v", got)
	}
	if got := eventTypesOf(delivered(ems, w.gm, w.other)); len(got) != 1 || got[0] != "player.left" {
		t.Fatalf("the other player received %v, want only player.left", got)
	}

	if w.s.Player(testPlayerID) != nil {
		t.Fatal("the kicked player is still seated")
	}
	if w.s.Pawn(pawn) == nil {
		t.Fatal("the kicked player's pawn was removed from the board with them")
	}
}

// A disconnect is not a departure. The row stays so that the reconnect is one
// field changing back, with the player's pawns and turn where they were.
func TestADisconnectKeepsThePlayerSeated(t *testing.T) {
	w := newWorld(t)

	ems := w.apply(&PlayerSetConnected{ID: testPlayerID, Connected: false}, w.gm)
	equalStrings(t, "emissions", summary(ems), []string{"player.updated to all"})

	p := w.s.Player(testPlayerID)
	if p == nil {
		t.Fatal("a disconnect removed the player")
	}
	if p.Connected {
		t.Fatal("the player is still marked connected")
	}

	// And coming back is an update rather than a join, so nobody's client
	// treats a reconnect as a new arrival.
	back := w.apply(&PlayerJoin{Player: Player{ID: testPlayerID, Name: "Ari", Role: RolePlayer}}, w.gm)
	equalStrings(t, "emissions", summary(back), []string{"player.updated to all"})
}

// A ROOM SAVED WHEN THE SETTING WAS CALLED SOMETHING ELSE comes back with a
// value nothing accepts, because the field it was written into no longer
// exists. The room is otherwise intact, so the one field is repaired: the
// alternative is a settings window with no radio selected and a refusal on the
// next unrelated change the GM makes.
func TestNormalizeRepairsALabelSettingThatNoLongerExists(t *testing.T) {
	s := NewState(testRoomID, "The Sunless Citadel", Env{})

	// "band" is what every room on the old wording holds, and the empty string
	// is what a snapshot written before the field existed unmarshals to.
	for _, stale := range []PawnLabels{"band", "hidden", "exact", ""} {
		s.Table.PawnLabels = stale
		s.Normalize()

		if s.Table.PawnLabels != LabelsDefault {
			t.Fatalf("%q came back as %q, want %q", stale, s.Table.PawnLabels, LabelsDefault)
		}
	}

	// And a setting that IS one is left exactly as the GM chose it.
	for _, live := range []PawnLabels{LabelsNone, LabelsDefault, LabelsFull} {
		s.Table.PawnLabels = live
		s.Normalize()

		if s.Table.PawnLabels != live {
			t.Fatalf("a live setting was rewritten from %q to %q", live, s.Table.PawnLabels)
		}
	}
}
