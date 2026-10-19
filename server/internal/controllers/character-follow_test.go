package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
func panelFragment(t *testing.T, app *App, query string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	return tableRequest(t, app.CharacterPanelFragment, http.MethodGet, "/fragment/character/panel?"+query, nil, nil, sess)
}
func seatedPawn(hp, maxHP, ac int, size room.Size) *room.Pawn {
	character := testCharacterID
	return &room.Pawn{
		Kind: room.PawnPlayer, CharacterID: &character, Size: size,
		HP: &hp, MaxHP: &maxHP, AC: &ac,
	}
}
func TestASheetOpenedBeforeJoiningFollowsTheRoomOnceItsCharacterSits(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 12, 12)}}
	page := standalonePage(t, newRoomApp(db), sheetSession(false))
	for _, section := range pages.LiveSheetSections() {
		want := `hx-get="` + strings.ReplaceAll(pages.CharacterPanelPath(testCharacterID.String(), section), "&", "&amp;") + `"`
		if !strings.Contains(page, want) {
			t.Errorf("the %s panel does not refetch itself: no %s", section, want)
		}
	}
	if !strings.Contains(page, pages.SheetFollowScript) {
		t.Error("the page never listens for the room tab")
	}
	if !strings.Contains(page, "detail.id === &#39;"+testCharacterID.String()+"&#39;") {
		t.Error("the panels answer a change to any character, not only this one")
	}
	db = &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 12, 12)}}
	app := newRoomApp(db)
	app.Hub = seatedHub{pawn: seatedPawn(5, 12, 15, room.SizeMedium)}
	rec := panelFragment(t, app, "character="+testCharacterID.String()+"&section="+pages.SheetSectionVitals, sheetSession(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if tag := tagByID(t, rec.Body.String(), "current_hp"); !strings.Contains(tag, `value="5"`) {
		t.Errorf("after joining, the refresh does not show the pawn's 5 hit points\n%s", tag)
	}
}
func TestThePanelFragmentShowsThePawnOnlyForTheCharacterTheSessionPlays(t *testing.T) {
	elsewhere := sheetSession(true)
	other := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS9")
	elsewhere.CharacterID = &other
	for name, sess := range map[string]session.UserSession{
		"no room":                        sheetSession(false),
		"another character at the table": elsewhere,
	} {
		db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 12, 12)}}
		app := newRoomApp(db)
		app.Hub = seatedHub{pawn: seatedPawn(5, 12, 15, room.SizeMedium)}
		rec := panelFragment(t, app, "character="+testCharacterID.String()+"&section="+pages.SheetSectionVitals, sess)
		if tag := tagByID(t, rec.Body.String(), "current_hp"); !strings.Contains(tag, `value="12"`) {
			t.Errorf("%s: the panel shows a pawn this character is not playing\n%s", name, tag)
		}
	}
}
func TestThePanelFragmentIsRefusedWithAnEmptyBody(t *testing.T) {
	id := testCharacterID.String()
	for name, c := range map[string]struct {
		query  string
		stored roomAnswer
	}{
		"a character that is not a ulid": {"character=nonsense&section=vitals", storedCharacterAnswer("medium", 15, 12, 12)},
		"no character at all":            {"section=vitals", storedCharacterAnswer("medium", 15, 12, 12)},
		"a section that is a whole tab":  {"character=" + id + "&section=main", storedCharacterAnswer("medium", 15, 12, 12)},
		"a section nobody serves":        {"character=" + id + "&section=spellbook", storedCharacterAnswer("medium", 15, 12, 12)},
		"no section at all":              {"character=" + id, storedCharacterAnswer("medium", 15, 12, 12)},
		"a character somebody else owns": {"character=" + id + "&section=vitals", roomAnswer{}},
	} {
		db := &roomDB{rows: 1, answers: []roomAnswer{c.stored}}
		rec := panelFragment(t, newRoomApp(db), c.query, sheetSession(false))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, rec.Code)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "" {
			t.Errorf("%s: the refusal carries a body: %q", name, body)
		}
	}
}
func TestTheSheetInATabShowsThePawnWhileTheRoomHoldsOne(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("large", 18, 3, 30)}}
	app := newRoomApp(db)
	app.Hub = seatedHub{pawn: seatedPawn(12, 40, 15, room.SizeMedium)}
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
	db := &roomDB{rows: 1, answers: []roomAnswer{storedCharacterAnswer("medium", 15, 7, 12)}}
	rec := panelFragment(t, newRoomApp(db), "character="+testCharacterID.String()+"&section="+pages.SheetSectionVitals, sheetSession(true))
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
