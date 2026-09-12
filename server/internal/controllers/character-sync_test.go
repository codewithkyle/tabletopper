package controllers

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func characterForRoomAnswer(name string, size string, ac, hp, maxHP int) roomAnswer {
	return roomAnswer{
		columns: []string{"id", "owner_id", "name", "size", "ac", "current_hp", "max_hp", "initiative_bonus", "asset_id"},
		values: []driver.Value{
			testCharacterID.Bytes(), testOwnerID.Bytes(), name, size,
			int64(ac), int64(hp), int64(maxHP), int64(2), nil,
		},
	}
}
func roomWithASeatedPawn(t *testing.T) json.RawMessage {
	t.Helper()
	s := room.NewState(testRoomID, "Curse of Strahd", room.Env{})
	hp, maxHP, ac := 12, 12, 15
	character, owner := testCharacterID, testOwnerID
	s.Players = append(s.Players, room.Player{
		ID: owner, Name: "Ari", Role: room.RolePlayer,
		CharacterID: &character, CharacterName: "Ilyana",
	})
	s.Pawns = append(s.Pawns, room.Pawn{
		ID: testPawnA, Kind: room.PawnPlayer, Name: "Ilyana", Size: room.SizeMedium,
		LayerID: s.Table.ActiveLayer, X: 64, Y: 64, Visible: true,
		HP: &hp, MaxHP: &maxHP, AC: &ac,
		OwnerID: &owner, CharacterID: &character,
	})
	blob, err := room.Marshal(s)
	if err != nil {
		t.Fatalf("marshalling the room: %v", err)
	}
	return blob
}
func sheetSession(inRoom bool) session.UserSession {
	sess := session.UserSession{UserID: testOwnerID, Hash: []byte("session-hash")}
	character := testCharacterID
	sess.CharacterID = &character
	if inRoom {
		id := testRoomID
		sess.RoomID = &id
	}
	return sess
}
func vitalsForm() url.Values {
	return url.Values{
		"max_hp": {"24"}, "current_hp": {"7"}, "temp_hp": {"0"},
		"hit_dice": {"3d8"}, "hit_dice_spent": {"0"}, "exhaustion": {"0"},
	}
}
func sheetPost(t *testing.T, app *App, handler http.HandlerFunc, form url.Values, sess session.UserSession) {
	t.Helper()
	tableRequest(t, handler, http.MethodPost,
		"/characters/"+testCharacterID.String()+"/vitals",
		map[string]string{"id": testCharacterID.String()}, form, sess)
}
func TestASheetSaveInARoomReachesThePawn(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{characterForRoomAnswer("Ilyana Duskhollow", "large", 18, 7, 24)}}
	app := newRoomApp(db)
	app.Hub = hub.New(app.Queries, hub.Options{Store: storedRoom{snapshot: roomWithASeatedPawn(t)}})
	firstLayer(t, app)
	sheetPost(t, app, app.SaveCharacterVitals, vitalsForm(), sheetSession(true))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pawn, ok := app.Hub.Pawn(ctx, testRoomID, testPawnA, room.RoleGM)
	if !ok {
		t.Fatal("the pawn is no longer on the table")
	}
	if *pawn.HP != 7 || *pawn.MaxHP != 24 || *pawn.AC != 18 || pawn.Size != room.SizeLarge {
		t.Errorf("the pawn is %d/%d ac %d %s, want the sheet's row", *pawn.HP, *pawn.MaxHP, *pawn.AC, pawn.Size)
	}
	if pawn.Name != "Ilyana Duskhollow" {
		t.Errorf("the pawn is still called %q", pawn.Name)
	}
}
func TestASheetSaveOutsideARoomReadsNothingBack(t *testing.T) {
	db := &roomDB{rows: 1, answers: []roomAnswer{characterForRoomAnswer("Ilyana", "medium", 15, 12, 12)}}
	app := newRoomApp(db)
	app.Hub = hub.New(app.Queries, hub.Options{Store: storedRoom{snapshot: roomWithASeatedPawn(t)}})
	sheetPost(t, app, app.SaveCharacterVitals, vitalsForm(), sheetSession(false))
	for _, query := range db.queries() {
		if strings.Contains(query, "GetCharacterForRoom") {
			t.Errorf("a sheet save with no room behind it still read the character back for a pawn: %v", db.queries())
		}
	}
	if app.Hub.Loaded() != 0 {
		t.Errorf("a sheet save with no room behind it loaded %d rooms", app.Hub.Loaded())
	}
}
func TestASheetSaveIsHeldToTheSameLimitsAsThePawn(t *testing.T) {
	for name, form := range map[string]url.Values{
		"maximum hit points": withValue(vitalsForm(), "max_hp", "60000"),
		"current hit points": withValue(vitalsForm(), "current_hp", "60000"),
	} {
		app, db := newPanelApp(1)
		rec := panelPost(t, db, app.SaveCharacterVitals, form, map[string]string{"id": testCharacterID.String()})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s of 60000 was accepted with %d; the room caps at %d", name, rec.Code, room.HPLimit)
		}
	}
	app, db := newPanelApp(1)
	rec := panelPost(t, db, app.SaveCharacterCoreStats, url.Values{
		"xp": {"0"}, "speed": {"30 ft."}, "ac": {"400"},
		"initiative_bonus": {"0"}, "spellcasting_ability": {"none"}, "spell_bonus_misc": {"0"},
	}, map[string]string{"id": testCharacterID.String()})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("an armour class of 400 was accepted with %d; the room caps at %d", rec.Code, room.ACLimit)
	}
}
func withValue(form url.Values, key, value string) url.Values {
	out := url.Values{}
	for k, v := range form {
		out[k] = append([]string(nil), v...)
	}
	out.Set(key, value)
	return out
}

func sheetFragment(t *testing.T, app *App, query string, sess session.UserSession) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/fragment/character/sheet?"+query, nil)
	r = r.WithContext(session.NewContext(r.Context(), sess))
	rec := httptest.NewRecorder()
	app.CharacterSheetFragment(rec, r)
	return rec
}
func TestTheSheetWindowIsRefusedWithAnEmptyBody(t *testing.T) {
	inRoom := sheetSession(true)
	noCharacter := sheetSession(true)
	noCharacter.CharacterID = nil
	elsewhere := sheetSession(true)
	other := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVS9")
	elsewhere.RoomID = &other
	for name, c := range map[string]struct {
		query string
		sess  session.UserSession
	}{
		"no character on the session":  {"room=" + testRoomID.String(), noCharacter},
		"a room the session is not in": {"room=" + testRoomID.String(), elsewhere},
		"a room that is not a ulid":    {"room=nonsense", inRoom},
		"no room at all":               {"", inRoom},
		"a section nobody serves":      {"room=" + testRoomID.String() + "&section=spellbook", inRoom},
		"a spell level above the nine": {"room=" + testRoomID.String() + "&section=spells&level=12", inRoom},
		"a spell level that is a word": {"room=" + testRoomID.String() + "&section=spells&level=cantrips", inRoom},
		"a spell level below zero":     {"room=" + testRoomID.String() + "&section=spells&level=-1", inRoom},
	} {
		db := &roomDB{rows: 1, answers: []roomAnswer{tableRoomAnswer()}}
		app := newRoomApp(db)
		app.Hub = hub.New(app.Queries, hub.Options{Store: storedRoom{snapshot: roomWithASeatedPawn(t)}})
		rec := sheetFragment(t, app, c.query, c.sess)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, rec.Code)
		}
		if body := strings.TrimSpace(rec.Body.String()); body != "" {
			t.Errorf("%s: the refusal carries a body: %q", name, body)
		}
	}
}

func TestThePawnIsWhatTheSheetShowsWhileTheRoomHoldsOne(t *testing.T) {
	row := func() pages.EditCharacterPageData {
		return pages.EditCharacterPageData{
			CurrentHP: "12", MaxHP: "12", AC: "15", Size: "medium",
			Header: pages.CharacterHeader{CurrentHP: "12", MaxHP: "12", AC: "15"},
		}
	}
	hp, maxHP, ac := 3, 24, 18
	character := testCharacterID
	pawn := &room.Pawn{
		Kind: room.PawnPlayer, CharacterID: &character, Size: room.SizeLarge,
		HP: &hp, MaxHP: &maxHP, AC: &ac,
	}
	data := row()
	applyPawnVitals(&data, pawn)
	if data.CurrentHP != "3" || data.MaxHP != "24" || data.AC != "18" || data.Size != "large" {
		t.Errorf("the sheet reads %s/%s ac %s %s, want the pawn's", data.CurrentHP, data.MaxHP, data.AC, data.Size)
	}
	if data.Header.CurrentHP != "3" || data.Header.MaxHP != "24" || data.Header.AC != "18" {
		t.Errorf("the chips read %s/%s ac %s, want the pawn's", data.Header.CurrentHP, data.Header.MaxHP, data.Header.AC)
	}
	for name, absent := range map[string]*room.Pawn{
		"no pawn at all":                    nil,
		"a pawn the room has no vitals for": {Kind: room.PawnPlayer, CharacterID: &character},
	} {
		data := row()
		applyPawnVitals(&data, absent)
		if data.CurrentHP != "12" || data.MaxHP != "12" || data.AC != "15" || data.Size != "medium" {
			t.Errorf("%s: the sheet was overwritten with %s/%s ac %s %s", name, data.CurrentHP, data.MaxHP, data.AC, data.Size)
		}
	}
}
