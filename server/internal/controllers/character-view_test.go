package controllers

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func characterSheetPath(character string, section string) string {
	path := "/fragment/room/character-sheet?room=" + testRoomID.String() + "&character=" + character
	if section != "" {
		path += "&section=" + section
	}
	return path
}
func TestTheCharacterSheetRouteIsAnEmpty404ForAnythingButTheGMsOwnTable(t *testing.T) {
	for name, c := range map[string]struct {
		path string
		asGM bool
	}{
		"a player":                       {path: characterSheetPath(testCharacterID.String(), "")},
		"a character nobody brought":     {path: characterSheetPath(testCharacterID.String(), ""), asGM: true},
		"a character id that is not one": {path: characterSheetPath("not-a-ulid", ""), asGM: true},
		"the journal":                    {path: characterSheetPath(testCharacterID.String(), "journal"), asGM: true},
		"a section that does not exist":  {path: characterSheetPath(testCharacterID.String(), "secrets"), asGM: true},
		"a spell level past ninth":       {path: characterSheetPath(testCharacterID.String(), "spells") + "&level=10", asGM: true},
	} {
		t.Run(name, func(t *testing.T) {
			app := tableApp(t, &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}})
			sess := memberSession(testRoomID)
			if c.asGM {
				sess = gmSession()
			}
			rec := tableRequest(t, app.RoomCharacterSheetFragment, http.MethodGet, c.path, nil, nil, sess)
			if rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("the refusal carried a body: %s", rec.Body.String())
			}
		})
	}
}
func TestEachViewTabReadsOnlyWhatItShowsAndAsTheCharactersOwner(t *testing.T) {
	character := queries.Character{ID: testCharacterID, OwnerID: testOwnerID}
	for _, c := range []struct {
		section string
		level   int
		want    string
	}{
		{pages.SheetSectionMain, pages.SpellsPrepared, ""},
		{pages.SheetSectionAbilities, pages.SpellsPrepared, ""},
		{pages.SheetSectionSkills, pages.SpellsPrepared, ""},
		{pages.SheetSectionAttacks, pages.SpellsPrepared, "-- name: ListCharacterAttacks"},
		{pages.SheetSectionInventory, pages.SpellsPrepared, "-- name: ListCharacterInventory"},
		{pages.SheetSectionSpells, pages.SpellsPrepared, "-- name: ListPreparedSpells"},
		{pages.SheetSectionSpells, 3, "-- name: GetSpellSlots"},
	} {
		app, db := newPanelApp(1)
		_, _ = app.characterView(context.Background(), character, c.section, c.level)
		statements := append(append([]recordedCall{}, db.reads...), db.calls...)
		if c.want == "" {
			if len(statements) != 0 {
				t.Errorf("the %s tab ran statements beyond the character it was handed", c.section)
			}
			continue
		}
		if len(statements) == 0 || !strings.Contains(statements[0].query, c.want) {
			t.Errorf("the %s tab at level %d did not start with %s: %v", c.section, c.level, c.want, statements)
			continue
		}
		if !slices.Contains(statements[0].args, any(testOwnerID)) {
			t.Errorf("the %s tab read as somebody other than the character's owner: %v", c.section, statements[0].args)
		}
	}
}
func TestTheViewKeepsThePortraitAShareLinkHides(t *testing.T) {
	asset := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS5")
	app, _ := newPanelApp(1)
	data, err := app.characterView(context.Background(), queries.Character{ID: testCharacterID, OwnerID: testOwnerID, AssetID: &asset}, pages.SheetSectionMain, pages.SpellsPrepared)
	if err != nil {
		t.Fatal(err)
	}
	if data.Sheet.Header.AvatarID != asset.String() {
		t.Errorf("the GM's view has portrait %q, want %q", data.Sheet.Header.AvatarID, asset.String())
	}
}
func TestTheViewReadsHitPointsOffThePawnOnTheTable(t *testing.T) {
	hp, maxHP, ac := 7, 31, 17
	character := queries.Character{CurrentHP: 31, MaxHP: 31, AC: 15, Size: "medium"}
	pawnVitalsOnCharacter(&character, &room.Pawn{
		Kind:        room.PawnPlayer,
		CharacterID: &testCharacterID,
		HP:          &hp,
		MaxHP:       &maxHP,
		AC:          &ac,
		Size:        room.SizeLarge,
	})
	if character.CurrentHP != 7 || character.MaxHP != 31 || character.AC != 17 || character.Size != string(room.SizeLarge) {
		t.Errorf("the sheet reads %d/%d AC %d %s, want the pawn's 7/31 AC 17 large", character.CurrentHP, character.MaxHP, character.AC, character.Size)
	}
	alone := queries.Character{CurrentHP: 12, MaxHP: 20, AC: 14, Size: "small"}
	pawnVitalsOnCharacter(&alone, nil)
	if alone.CurrentHP != 12 || alone.MaxHP != 20 || alone.AC != 14 || alone.Size != "small" {
		t.Errorf("a character with no pawn changed to %d/%d AC %d %s", alone.CurrentHP, alone.MaxHP, alone.AC, alone.Size)
	}
}
func TestOnlyTheGMIsToldWhichCharacterAPawnIs(t *testing.T) {
	pawn := &room.Pawn{Kind: room.PawnPlayer, Name: "Ilyana", CharacterID: &testCharacterID}
	if got := pawnView(pawn, room.RoleGM, room.LabelsDefault, "").CharacterID; got != testCharacterID.String() {
		t.Errorf("the GM's panel names character %q", got)
	}
	if got := pawnView(pawn, room.RolePlayer, room.LabelsDefault, "").CharacterID; got != "" {
		t.Errorf("a player's panel names character %q", got)
	}
	npc := &room.Pawn{Kind: room.PawnNPC, Name: "Barkeep", CharacterID: &testCharacterID}
	if got := pawnView(npc, room.RoleGM, room.LabelsDefault, "").CharacterID; got != "" {
		t.Errorf("an NPC's panel names character %q", got)
	}
}
