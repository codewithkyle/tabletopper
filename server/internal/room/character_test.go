package room

import (
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestOnlyASeatedPawnCarriesVitalsBackToItsSheet(t *testing.T) {
	character := testCharID
	seated := func() Pawn {
		hp, maxHP, ac := 9, 12, 15
		return Pawn{
			Kind:        PawnPlayer,
			CharacterID: &character,
			Size:        SizeMedium,
			HP:          &hp,
			MaxHP:       &maxHP,
			AC:          &ac,
		}
	}
	if v, ok := PawnVitals(seated()); !ok || v != (SheetVitals{HP: 9, MaxHP: 12, AC: 15, Size: SizeMedium}) {
		t.Fatalf("a seated pawn gave %+v, %v", v, ok)
	}
	for name, spoil := range map[string]func(*Pawn){
		"a monster":           func(p *Pawn) { p.Kind = PawnMonster },
		"an npc":              func(p *Pawn) { p.Kind = PawnNPC },
		"an object":           func(p *Pawn) { p.Kind = PawnObject },
		"nobody's character":  func(p *Pawn) { p.CharacterID = nil },
		"no hit points":       func(p *Pawn) { p.HP = nil },
		"no maximum":          func(p *Pawn) { p.MaxHP = nil },
		"no armour class":     func(p *Pawn) { p.AC = nil },
		"not a creature size": func(p *Pawn) { p.Size = "" },
	} {
		p := seated()
		spoil(&p)
		if v, ok := PawnVitals(p); ok {
			t.Errorf("%s offered the sheet %+v", name, v)
		}
	}
}

func testCharacterInfo() CharacterInfo {
	return CharacterInfo{
		ID:      testCharID,
		OwnerID: testPlayerID,
		Name:    "Ilyana Duskhollow",
		Size:    SizeLarge,
		HP:      7,
		MaxHP:   24,
		AC:      18,
		Image:   "/assets/images/" + testAssetID.String(),
	}
}
func (w *world) seatedPawn() ulid.ULID {
	w.t.Helper()
	hp, maxHP, ac := 12, 12, 15
	return w.spawn(Pawn{
		Kind: PawnPlayer, CharacterID: &testCharID, OwnerID: &testPlayerID,
		Name: "Ilyana", Size: SizeMedium, Visible: true,
		X: 128, Y: 192, HP: &hp, MaxHP: &maxHP, AC: &ac,
	})
}

func TestASheetSaveMovesItsPawnAndLeavesTheRestOfItAlone(t *testing.T) {
	w := newWorld(t)
	id := w.seatedPawn()
	upstairs := w.addLayer("Upstairs")
	w.apply(&PawnSetLayer{IDs: []ulid.ULID{id}, Layer: upstairs}, w.gm)
	w.apply(&PawnSetConditions{ID: id, Conditions: []Condition{
		{Name: "Poisoned", Color: ColorGreen, Duration: -1, Clear: ClearEnd},
	}}, w.gm)
	before := *w.s.Pawn(id)
	w.apply(&CharacterSync{Info: testCharacterInfo()}, Actor{})
	p := w.s.Pawn(id)
	if p.Name != "Ilyana Duskhollow" || p.Size != SizeLarge {
		t.Errorf("the pawn is %q at %s, want the sheet's name and size", p.Name, p.Size)
	}
	if *p.HP != 7 || *p.MaxHP != 24 || *p.AC != 18 {
		t.Errorf("the pawn is %d/%d ac %d, want 7/24 ac 18", *p.HP, *p.MaxHP, *p.AC)
	}
	if p.Image != testCharacterInfo().Image {
		t.Errorf("the pawn's portrait is %q", p.Image)
	}
	if p.X != before.X || p.Y != before.Y || p.Z != before.Z {
		t.Errorf("the sheet moved the pawn from (%d,%d,%d) to (%d,%d,%d)", before.X, before.Y, before.Z, p.X, p.Y, p.Z)
	}
	if p.LayerID != upstairs || p.Visible != before.Visible || len(p.Conditions) != 1 {
		t.Errorf("the sheet changed the floor, the visibility or the conditions: %+v", p)
	}
	if seat := w.s.Player(testPlayerID); seat.CharacterName != "Ilyana Duskhollow" {
		t.Errorf("the seat still reads %q", seat.CharacterName)
	}
}
func TestASheetSaveClampsHitPointsToTheMaximumItBrings(t *testing.T) {
	w := newWorld(t)
	id := w.seatedPawn()
	info := testCharacterInfo()
	info.MaxHP, info.HP = 5, 30
	w.apply(&CharacterSync{Info: info}, Actor{})
	if p := w.s.Pawn(id); *p.HP != 5 || *p.MaxHP != 5 {
		t.Errorf("the pawn is %d/%d, want 5/5", *p.HP, *p.MaxHP)
	}
}
func TestASheetSaveIsHeldToTheRoomsOwnLimits(t *testing.T) {
	w := newWorld(t)
	id := w.seatedPawn()
	info := testCharacterInfo()
	info.MaxHP, info.HP, info.AC = 60_000, 60_000, 400
	w.apply(&CharacterSync{Info: info}, Actor{})
	p := w.s.Pawn(id)
	if *p.MaxHP != HPLimit || *p.HP != HPLimit || *p.AC != ACLimit {
		t.Errorf("the pawn is %d/%d ac %d, want %d/%d ac %d", *p.HP, *p.MaxHP, *p.AC, HPLimit, HPLimit, ACLimit)
	}
}
func TestASheetSaveWithNoPawnOnTheTableStillRenamesTheSeat(t *testing.T) {
	w := newWorld(t)
	w.apply(&CharacterSync{Info: testCharacterInfo()}, Actor{})
	if seat := w.s.Player(testPlayerID); seat.CharacterName != "Ilyana Duskhollow" {
		t.Errorf("the seat still reads %q", seat.CharacterName)
	}
	if len(w.s.Pawns) != 0 {
		t.Errorf("a sheet save put %d pawns on the table", len(w.s.Pawns))
	}
}
func TestASheetSaveBorrowsTheSeatAvatarWhenTheCharacterHasNoPortrait(t *testing.T) {
	w := newWorld(t)
	w.s.Player(testPlayerID).Avatar = "/assets/images/seat.webp"
	id := w.seatedPawn()
	info := testCharacterInfo()
	info.Image = ""
	w.apply(&CharacterSync{Info: info}, Actor{})
	if p := w.s.Pawn(id); p.Image != "/assets/images/seat.webp" {
		t.Errorf("the pawn's portrait is %q, want the seat's", p.Image)
	}
}
func TestASheetSaveChangesNothingWhenTheSheetAlreadyAgrees(t *testing.T) {
	w := newWorld(t)
	w.seatedPawn()
	info := testCharacterInfo()
	w.apply(&CharacterSync{Info: info}, Actor{})
	ch := w.change(&CharacterSync{Info: info}, Actor{})
	if got := ch.changes(RoleGM); len(got) != 0 {
		t.Errorf("a second identical save derived %d changes, want none", len(got))
	}
}

func TestASheetSaveReachesTheTableAndThePlayerWhoseSheetItIs(t *testing.T) {
	w := newWorld(t)
	id := w.seatedPawn()
	info := testCharacterInfo()
	info.HP, info.MaxHP = 1, 24
	ch := w.change(&CharacterSync{Info: info}, Actor{})
	for _, viewer := range []Actor{w.gm, w.pc, w.other} {
		var moved *Pawn
		for _, c := range ch.changes(viewer.Role) {
			upserted, ok := c.(*PawnsUpserted)
			if !ok {
				continue
			}
			for i, p := range upserted.Pawns {
				if p.ID == id {
					moved = &upserted.Pawns[i]
				}
			}
		}
		if moved == nil {
			t.Fatalf("the %s was not told the pawn moved", viewer.Role)
		}
		if moved.HP == nil || *moved.HP != 1 || moved.MaxHP == nil || *moved.MaxHP != 24 {
			t.Errorf("the %s was told %v/%v, want 1/24", viewer.Role, moved.HP, moved.MaxHP)
		}
		band := Health(*moved)
		if band == nil || *band != BandNearDeath {
			t.Errorf("the %s reads the band as %v, want near death", viewer.Role, band)
		}
	}
}
