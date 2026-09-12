package room

import (
	"testing"
)

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
	for _, p := range players.Pawns {
		if p.ID == hidden {
			t.Fatal("the hidden pawn is in the players' copy")
		}
	}
}
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
				if monster && !tc.exact && p.AC != nil {
					t.Fatalf("a %s pawn kept its armour class under %q", p.Kind, tc.setting)
				}
				if (!monster || tc.exact) && (p.AC == nil || *p.AC != 15) {
					t.Fatalf("a %s pawn lost its armour class under %q", p.Kind, tc.setting)
				}
			}
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
func TestExactHitPointsAreShownToTheRightViewers(t *testing.T) {
	settings := []PawnLabels{LabelsNone, LabelsDefault, LabelsFull}
	for _, labels := range settings {
		for _, kind := range []PawnKind{PawnPlayer, PawnMonster, PawnNPC, PawnObject} {
			if !ExactHP(kind, labels, RoleGM) {
				t.Fatalf("the GM was refused a %s pawn's hit points under %q", kind, labels)
			}
		}
	}
	for _, labels := range settings {
		for _, kind := range []PawnKind{PawnPlayer, PawnObject} {
			if !ExactHP(kind, labels, RolePlayer) {
				t.Fatalf("a player was refused a %s pawn's hit points under %q", kind, labels)
			}
		}
	}
	for _, kind := range []PawnKind{PawnMonster, PawnNPC} {
		if ExactHP(kind, LabelsNone, RolePlayer) || ExactHP(kind, LabelsDefault, RolePlayer) {
			t.Fatalf("a player was shown a %s's hit points in a room that labels words", kind)
		}
		if !ExactHP(kind, LabelsFull, RolePlayer) {
			t.Fatalf("a player was refused a %s's hit points in an open room", kind)
		}
	}
}
func TestTheHealthBandsSitWhereTheyAreDescribed(t *testing.T) {
	tests := []struct {
		hp, maxHP int
		want      HPBand
	}{
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
		{20, 20, BandHealthy},
		{16, 20, BandHealthy},
		{15, 20, BandBruised},
		{10, 20, BandBloody},
		{5, 20, BandVeryBloody},
		{2, 20, BandVeryBloody},
		{1, 20, BandNearDeath},
		{0, 20, BandDead},
		{7, 7, BandHealthy},
		{6, 7, BandHealthy},
		{5, 7, BandBruised},
		{4, 7, BandBruised},
		{3, 7, BandBloody},
		{2, 7, BandBloody},
		{1, 7, BandVeryBloody},
		{1, 1000, BandNearDeath},
		{0, 1000, BandDead},
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
func TestAKickTellsTheTargetAndTheRoomDifferentThings(t *testing.T) {
	w := newWorld(t)
	pawn := w.spawn(Pawn{Kind: PawnPlayer, Name: "Ari", Visible: true, OwnerID: &testPlayerID})
	ch := w.change(&PlayerKick{ID: testPlayerID}, w.gm)
	equalStrings(t, "signals", summary(ch.signals), []string{"player.kicked to player"})
	equalStrings(t, "the room", changeTypesOf(ch.changes(RoleGM)), []string{"players.removed"})
	equalStrings(t, "the kicked player", eventTypesOf(ch.sent(w.pc)), []string{"changes", "player.kicked"})
	equalStrings(t, "the other player", eventTypesOf(ch.sent(w.other)), []string{"changes"})
	if w.s.Player(testPlayerID) != nil {
		t.Fatal("the kicked player is still seated")
	}
	if w.s.Pawn(pawn) == nil {
		t.Fatal("the kicked player's pawn was removed from the board with them")
	}
}
func TestADisconnectKeepsThePlayerSeated(t *testing.T) {
	w := newWorld(t)
	ch := w.change(&PlayerSetConnected{ID: testPlayerID, Connected: false}, w.gm)
	equalStrings(t, "a disconnect", changeTypesOf(ch.changes(RoleGM)), []string{"players.upserted"})
	p := w.s.Player(testPlayerID)
	if p == nil {
		t.Fatal("a disconnect removed the player")
	}
	if p.Connected {
		t.Fatal("the player is still marked connected")
	}
	back := w.change(&PlayerJoin{Player: Player{ID: testPlayerID, Name: "Ari", Role: RolePlayer}}, w.gm)
	equalStrings(t, "a return", changeTypesOf(back.changes(RoleGM)), []string{"players.upserted"})
}
func TestNormalizeRepairsALabelSettingThatNoLongerExists(t *testing.T) {
	s := NewState(testRoomID, "The Sunless Citadel", Env{})
	for _, stale := range []PawnLabels{"band", "hidden", "exact", ""} {
		s.Table.PawnLabels = stale
		s.Normalize()
		if s.Table.PawnLabels != LabelsDefault {
			t.Fatalf("%q came back as %q, want %q", stale, s.Table.PawnLabels, LabelsDefault)
		}
	}
	for _, live := range []PawnLabels{LabelsNone, LabelsDefault, LabelsFull} {
		s.Table.PawnLabels = live
		s.Normalize()
		if s.Table.PawnLabels != live {
			t.Fatalf("a live setting was rewritten from %q to %q", live, s.Table.PawnLabels)
		}
	}
}
