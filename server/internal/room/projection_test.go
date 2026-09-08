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

// THE THREE HIT POINT SETTINGS, against the three kinds of pawn they treat
// differently. A player's own character is never hidden from the table, and an
// object's hit points are the thing the party is currently hitting.
func TestMonsterHitPointsProjectByTheRoomsSetting(t *testing.T) {
	kinds := []PawnKind{PawnPlayer, PawnMonster, PawnNPC, PawnObject}

	tests := []struct {
		setting  HPVisibility
		numbers  bool
		banded   bool
		affected []PawnKind
	}{
		{HPExact, true, false, nil},
		{HPBandOn, false, true, []PawnKind{PawnMonster, PawnNPC}},
		{HPHidden, false, false, []PawnKind{PawnMonster, PawnNPC}},
	}

	for _, tc := range tests {
		t.Run(string(tc.setting), func(t *testing.T) {
			w := newWorld(t)
			w.apply(&TableSetOptions{MonsterHP: tc.setting, PlayersCanDraw: true}, w.gm)

			for _, kind := range kinds {
				p := Pawn{Kind: kind, Name: string(kind), Visible: true, HP: intp(5), MaxHP: intp(20), AC: intp(15)}
				if kind == PawnObject {
					p.Width, p.Height = DefaultCellSize, DefaultCellSize
				}
				w.spawn(p)
			}

			for _, p := range w.s.Project(RolePlayer).Pawns {
				affected := p.Kind == PawnMonster || p.Kind == PawnNPC

				switch {
				case !affected:
					if p.HP == nil || *p.HP != 5 || p.MaxHP == nil {
						t.Fatalf("a %s pawn lost its hit points under %q", p.Kind, tc.setting)
					}
					if p.HPBand != nil {
						t.Fatalf("a %s pawn was given a band under %q", p.Kind, tc.setting)
					}

				case tc.numbers:
					if p.HP == nil || *p.HP != 5 {
						t.Fatalf("a %s pawn lost its hit points under %q", p.Kind, tc.setting)
					}

				default:
					if p.HP != nil || p.MaxHP != nil {
						t.Fatalf("a %s pawn kept a number under %q", p.Kind, tc.setting)
					}
					if tc.banded && p.HPBand == nil {
						t.Fatalf("a %s pawn has no band under %q", p.Kind, tc.setting)
					}
					if !tc.banded && p.HPBand != nil {
						t.Fatalf("a %s pawn has a band under %q", p.Kind, tc.setting)
					}
				}
			}

			// The GM's copy never carries a band, whatever the setting: a band
			// is a thing that exists only in a projection.
			for _, p := range w.s.Project(RoleGM).Pawns {
				if p.HPBand != nil {
					t.Fatalf("the GM's copy of a %s pawn carries a band", p.Kind)
				}
			}
		})
	}
}

// The band thresholds are 5e's own vocabulary, and they are compared by
// multiplication so that an awkward maximum has exact boundaries rather than
// ones that depend on which way integer division fell.
func TestTheHealthBandsSitWhereTheyAreDescribed(t *testing.T) {
	tests := []struct {
		hp, maxHP int
		want      HPBand
	}{
		{20, 20, BandHealthy},
		{11, 20, BandHealthy},
		{10, 20, BandBloodied},
		{6, 20, BandBloodied},
		{5, 20, BandCritical},
		{1, 20, BandCritical},
		{0, 20, BandDead},

		// A maximum of 7: a quarter is 1.75 and a half is 3.5, so the
		// boundaries fall between whole numbers.
		{4, 7, BandHealthy},
		{3, 7, BandBloodied},
		{2, 7, BandBloodied},
		{1, 7, BandCritical},
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

	ems := w.apply(&TableSetOptions{MonsterHP: HPExact, PlayersCanDraw: true}, w.gm)

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
	again := w.apply(&TableSetOptions{MonsterHP: HPExact, PlayersCanDraw: false}, w.gm)
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
