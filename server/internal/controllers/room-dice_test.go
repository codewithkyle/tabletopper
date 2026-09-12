package controllers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/room"
	"tabletopper/internal/session"
)

const diceReads = 24

func diceApp(t *testing.T) *App {
	t.Helper()
	answers := make([]roomAnswer, 0, diceReads)
	for range diceReads {
		answers = append(answers, getRoomAnswer(testRoomID, testOwnerID, "Curse of Strahd", "AB2C", false, false))
	}
	return liveRoomApp(t, &roomDB{rows: 1, answers: answers})
}
func rollDice(t *testing.T, app *App, sess session.UserSession, form url.Values) int {
	t.Helper()
	rec := tableRequest(t, app.RollDice, http.MethodPost, "/rooms/"+testRoomID.String()+"/dice",
		map[string]string{"id": testRoomID.String()}, form, sess)
	return rec.Code
}
func tray(t *testing.T, app *App, sess session.UserSession) string {
	t.Helper()
	rec := tableRequest(t, app.RoomDiceFragment, http.MethodGet,
		"/fragment/room/dice?room="+testRoomID.String(), nil, nil, sess)
	if rec.Code != http.StatusOK {
		t.Fatalf("the tray returned %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestTheTrayIsFetchedByAnybodyInTheRoom(t *testing.T) {
	app := diceApp(t)
	for name, sess := range map[string]session.UserSession{
		"the GM":   {UserID: testOwnerID},
		"a player": memberSession(testRoomID),
	} {
		markup := tray(t, app, sess)
		if !strings.Contains(markup, `id="`+"room-dice-log"+`"`) {
			t.Errorf("%s was not sent the log:\n%s", name, markup)
		}
		if !strings.Contains(markup, `name="expr"`) {
			t.Errorf("%s was not sent anything to roll with:\n%s", name, markup)
		}
	}
}
func TestAnOpenRollReachesEverybodysTray(t *testing.T) {
	app := diceApp(t)
	form := url.Values{"expr": {"1d20 + 7"}, "label": {"Longsword"}, "adv": {"0"}}
	if code := rollDice(t, app, session.UserSession{UserID: testOwnerID}, form); code != http.StatusNoContent {
		t.Fatalf("the roll returned %d, want 204", code)
	}
	for name, sess := range map[string]session.UserSession{
		"the GM":   {UserID: testOwnerID},
		"a player": memberSession(testRoomID),
	} {
		if markup := tray(t, app, sess); !strings.Contains(markup, "Longsword") {
			t.Errorf("%s cannot see the roll:\n%s", name, markup)
		}
	}
}
func TestASecretRollIsOnlyInItsOwnRollersTray(t *testing.T) {
	app := diceApp(t)
	form := url.Values{"expr": {"1d20"}, "label": {"Stealth"}, "adv": {"0"}, "secret": {"1"}}
	if code := rollDice(t, app, memberSession(testRoomID), form); code != http.StatusNoContent {
		t.Fatalf("the roll returned %d, want 204", code)
	}
	if markup := tray(t, app, memberSession(testRoomID)); !strings.Contains(markup, "Stealth") {
		t.Errorf("the roller cannot see their own secret roll:\n%s", markup)
	}
	if markup := tray(t, app, session.UserSession{UserID: testOwnerID}); strings.Contains(markup, "Stealth") {
		t.Errorf("the GM was shown a player's secret roll:\n%s", markup)
	}
}
func TestABadExpressionIsRefusedIntoTheAlert(t *testing.T) {
	app := diceApp(t)
	form := url.Values{"expr": {"2d6 * 3"}, "adv": {"0"}}
	rec := tableRequest(t, app.RollDice, http.MethodPost, "/rooms/"+testRoomID.String()+"/dice",
		map[string]string{"id": testRoomID.String()}, form, session.UserSession{UserID: testOwnerID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Bad dice") {
		t.Errorf("the refusal did not reach the alert: %q", rec.Header().Get("HX-Trigger"))
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a refusal wrote a body: %s", rec.Body.String())
	}
}
func TestAnAdvantageValueOutsideTheThreeIsNotFound(t *testing.T) {
	app := diceApp(t)
	for _, value := range []string{"2", "-2", "high", "1.5"} {
		form := url.Values{"expr": {"1d20"}, "adv": {value}}
		rec := tableRequest(t, app.RollDice, http.MethodPost, "/rooms/"+testRoomID.String()+"/dice",
			map[string]string{"id": testRoomID.String()}, form, session.UserSession{UserID: testOwnerID})
		if rec.Code != http.StatusNotFound {
			t.Errorf("adv=%q returned %d, want 404", value, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("adv=%q wrote a body: %s", value, rec.Body.String())
		}
	}
}
func TestAdvantageRollsTheSecondDie(t *testing.T) {
	app := diceApp(t)
	form := url.Values{"expr": {"1d20"}, "adv": {"1"}}
	if code := rollDice(t, app, session.UserSession{UserID: testOwnerID}, form); code != http.StatusNoContent {
		t.Fatalf("the roll returned %d, want 204", code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	view, ok := app.Hub.Rolls(ctx, testRoomID, testOwnerID)
	if !ok {
		t.Fatal("the room would not answer with its rolls")
	}
	if len(view.Rolls) != 1 {
		t.Fatalf("the log holds %d rolls, want 1", len(view.Rolls))
	}
	r := view.Rolls[0]
	if r.Adv != room.AdvHigh || len(r.Dice) != 2 {
		t.Fatalf("adv = %d over %d dice, want %d over 2", r.Adv, len(r.Dice), room.AdvHigh)
	}
}
