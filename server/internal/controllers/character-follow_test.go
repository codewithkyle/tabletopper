package controllers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

type seatedHub struct {
	nopHub
	pawn *room.Pawn
}

func (h seatedHub) CharacterPawn(context.Context, ulid.ULID, ulid.ULID) (*room.Pawn, bool) {
	return h.pawn, h.pawn != nil
}
func standalonePage(t *testing.T, app *App, sess session.UserSession) string {
	t.Helper()
	rec := tableRequest(t, app.CharacterPage, http.MethodGet,
		"/characters/"+testCharacterID.String()+"/edit",
		map[string]string{"id": testCharacterID.String()}, nil, sess)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}
func TestTheSheetInATabFollowsTheRoomItsCharacterSitsIn(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 12, 12)}}
	page := standalonePage(t, newRoomApp(db), sheetSession(true))
	for _, section := range pages.LiveSheetSections() {
		want := `hx-get="` + strings.ReplaceAll(pages.SheetSectionPath(testRoomID.String(), section), "&", "&amp;") + `"`
		if !strings.Contains(page, want) {
			t.Errorf("the %s panel does not refetch itself: no %s", section, want)
		}
	}
	if !strings.Contains(page, "/js/sheet-follow.js") {
		t.Error("the page never listens for the room tab")
	}
	if !strings.Contains(page, "detail.id === &#39;"+testCharacterID.String()+"&#39;") {
		t.Error("the panels answer a change to any character, not only this one")
	}
}
func TestTheSheetInATabStaysStillWhenItsCharacterIsNotInTheRoom(t *testing.T) {
	elsewhere := sheetSession(true)
	other := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS9")
	elsewhere.CharacterID = &other
	for name, sess := range map[string]session.UserSession{
		"no room":                        sheetSession(false),
		"another character at the table": elsewhere,
	} {
		db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 12, 12)}}
		page := standalonePage(t, newRoomApp(db), sess)
		if strings.Contains(page, "/fragment/character/sheet") {
			t.Errorf("%s: the panels refetch from a room this character is not playing in", name)
		}
		if strings.Contains(page, "/js/sheet-follow.js") {
			t.Errorf("%s: the page listens for a room it is not following", name)
		}
	}
}
func TestTheSheetInATabShowsThePawnWhileTheRoomHoldsOne(t *testing.T) {
	hp, maxHP, ac := 12, 40, 15
	character := testCharacterID
	db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("large", 18, 3, 30)}}
	app := newRoomApp(db)
	app.Hub = seatedHub{pawn: &room.Pawn{
		Kind: room.PawnPlayer, CharacterID: &character, Size: room.SizeMedium,
		HP: &hp, MaxHP: &maxHP, AC: &ac,
	}}
	page := standalonePage(t, app, sheetSession(true))
	for id, want := range map[string]string{
		"current_hp": "12", "seen-current_hp": "12",
		"max_hp": "40", "seen-max_hp": "40",
		"ac": "15", "seen-ac": "15",
		"seen-size": "medium",
	} {
		tag := tagByID(t, page, id)
		if !strings.Contains(tag, `value="`+want+`"`) {
			t.Errorf("%s does not hold the pawn's %s\n%s", id, want, tag)
		}
	}
}
func TestALiveRefreshBringsTheHeaderAlong(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer(), storedCharacterAnswer("medium", 15, 7, 12)}}
	app := newRoomApp(db)
	rec := sheetFragment(t, app, "room="+testRoomID.String()+"&section="+pages.SheetSectionVitals, sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if tag := tagByID(t, body, "character-bar-chips"); !strings.Contains(tag, `hx-swap-oob="true"`) {
		t.Errorf("the header chips do not swap themselves in\n%s", tag)
	}
	if strings.Index(body, `id="character-bar-chips"`) < strings.Index(body, `id="current_hp"`) {
		t.Error("the header comes before the panel, so htmx swaps it into the panel's place")
	}
	chips := body[strings.Index(body, `id="character-bar-chips"`):]
	chips = chips[:strings.Index(chips, "</dl>")]
	if !strings.Contains(chips, ">7 <span") || !strings.Contains(chips, "/ 12") {
		t.Errorf("the header does not show the 7 of 12 hit points the table holds\n%s", chips)
	}
}
