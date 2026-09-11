package pages
import (
	"strings"
	"testing"
	"tabletopper/internal/room"
)
const (
	testTurnRoomID = "01BX5ZZKBKACTAV9WEVGEMMVW0"
	testTurnEntryA = "01BX5ZZKBKACTAV9WEVGEMMVW1"
	testTurnEntryB = "01BX5ZZKBKACTAV9WEVGEMMVW2"
	testTurnPawnA  = "01BX5ZZKBKACTAV9WEVGEMMVW3"
)
func turnStrip(isGM bool) RoomInitiativeData {
	return RoomInitiativeData{
		RoomID: testTurnRoomID,
		IsGM:   isGM,
		Entries: []RoomInitiativeEntry{
			{
				ID: testTurnEntryA, Name: "Ari", Kind: EntrySolo, Side: SidePlayer,
				Image: "/assets/ari.webp", Band: "bloody", Blood: "4",
				HP: "6 / 14", Active: true, Mine: !isGM, Solo: testTurnPawnA,
				Conditions: []RoomPawnCondition{
					{ID: "01COND", Name: "Blessed", Color: "yellow", DurationText: "2 turns"},
				},
			},
			{
				ID: testTurnEntryB, Name: "Goblin", Kind: EntrySolo, Side: SideMonster,
				Image: "/assets/goblin.webp", Band: "dead", Blood: "7",
				HP: "0 / 7", Hidden: isGM, Solo: "01BX5ZZKBKACTAV9WEVGEMMVW4",
			},
		},
	}
}
func TestTheStripsRefetchIsDeclaredInItsOwnMarkup(t *testing.T) {
	markup := decoded(t, RoomInitiative(turnStrip(true)))
	if !strings.Contains(markup, `hx-trigger="`+InitiativeTrigger+`"`) {
		t.Errorf("the strip's trigger is not %q:\n%s", InitiativeTrigger, markup)
	}
	if !strings.Contains(markup, `hx-sync="this:queue last"`) {
		t.Error("the strip does not queue its refetches")
	}
	if !strings.Contains(InitiativeTrigger, "data-dragging") {
		t.Error("the trigger does not decline while a line is being dragged")
	}
	if strings.Contains(markup, `hx-trigger="load`) {
		t.Error("the fragment's own root carries load")
	}
	if !strings.HasPrefix(InitiativeLoadTrigger, "load, ") {
		t.Error("the placeholder's trigger does not fetch once on load")
	}
}
func TestTheStripsFilterIsParseable(t *testing.T) {
	open := strings.Index(InitiativeTrigger, "[")
	shut := strings.Index(InitiativeTrigger, "]")
	if open < 0 || shut < open {
		t.Fatalf("the trigger has no filter in it: %q", InitiativeTrigger)
	}
	filter := InitiativeTrigger[open+1 : shut]
	if strings.ContainsAny(filter, "[,") {
		t.Errorf("the filter %q carries a bracket or a comma", filter)
	}
}
func TestAnEmptyTrackerRendersHidden(t *testing.T) {
	markup := html(t, RoomInitiative(RoomInitiativeData{RoomID: testTurnRoomID, Empty: true}))
	if !strings.Contains(markup, "hidden") {
		t.Errorf("an empty tracker is not hidden:\n%s", markup)
	}
}
func TestEveryFaceCarriesItsBand(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))
	for _, want := range []string{`data-band="bloody"`, `data-band="dead"`, `data-blood="4"`, `data-blood="7"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("the strip is missing %s:\n%s", want, markup)
		}
	}
	plain := turnStrip(true)
	plain.Entries[0].Band = ""
	plain.Entries[0].Blood = ""
	first := strings.SplitN(html(t, RoomInitiative(plain)), testTurnEntryB, 2)[0]
	if strings.Contains(first, "data-band") {
		t.Errorf("a creature with no band was given one:\n%s", first)
	}
}
func TestAGroupDrawsADotPerMemberAndThenACount(t *testing.T) {
	bands := make([]string, 0, InitiativePipMax)
	for range InitiativePipMax {
		bands = append(bands, "healthy")
	}
	pips, count := InitiativePips(bands)
	if len(pips) != InitiativePipMax || count != "" {
		t.Fatalf("%d members made %d dots and the count %q", len(bands), len(pips), count)
	}
	pips, count = InitiativePips(append(bands, "dead"))
	if len(pips) != 0 || count != "x13" {
		t.Fatalf("thirteen members made %d dots and the count %q", len(pips), count)
	}
	data := turnStrip(true)
	data.Entries[1].Kind = EntryGroup
	data.Entries[1].Pips = []RoomInitiativePip{{Band: "healthy"}, {Band: "dead"}}
	markup := html(t, RoomInitiative(data))
	if strings.Count(markup, "data-pip") != 2 {
		t.Errorf("the group did not draw one dot per member:\n%s", markup)
	}
	data.Entries[1].HP = ""
	data.Entries[1].Conditions = nil
}
func TestTheTimerAndEndTurnBelongToTheActingPlayerAlone(t *testing.T) {
	player := html(t, RoomInitiative(turnStrip(false)))
	if !strings.Contains(player, "data-turn-timer") || !strings.Contains(player, "End turn") {
		t.Errorf("the acting player was given no clock:\n%s", player)
	}
	if strings.Count(player, "data-turn-next") != 1 {
		t.Errorf("the player's screen has %d turn buttons, want one", strings.Count(player, "data-turn-next"))
	}
	other := turnStrip(false)
	other.Entries[0].Mine = false
	if markup := html(t, RoomInitiative(other)); strings.Contains(markup, "data-turn-timer") {
		t.Error("a player who is not acting was given the clock")
	}
	gm := html(t, RoomInitiative(turnStrip(true)))
	if strings.Contains(gm, "data-turn-timer") || strings.Contains(gm, "End turn") {
		t.Error("the GM was given a copy of the turn clock")
	}
	if strings.Count(gm, "data-turn-next") != 1 {
		t.Errorf("the GM's screen has %d turn buttons, want one", strings.Count(gm, "data-turn-next"))
	}
	if !strings.Contains(gm, "data-turn-next hidden") {
		t.Errorf("the GM's turn button is drawn on the strip:\n%s", gm)
	}
}
func TestTheStripsControlsAreTheGMsAlone(t *testing.T) {
	for name, isGM := range map[string]bool{"the GM": true, "a player": false} {
		markup := html(t, RoomInitiative(turnStrip(isGM)))
		hasDrag := strings.Contains(markup, "data-reorder")
		hasOrder := strings.Contains(markup, "data-turn-order")
		hasRemove := strings.Contains(markup, "data-entry-remove")
		if hasDrag != isGM || hasOrder != isGM || hasRemove != isGM {
			t.Errorf("%s: drag=%v order=%v remove=%v, want all %v", name, hasDrag, hasOrder, hasRemove, isGM)
		}
	}
}
func TestEachCardIsColouredByWhatItIs(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))
	for _, want := range []string{`data-side="player"`, `data-side="monster"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("the strip is missing %s:\n%s", want, markup)
		}
	}
	lair := turnStrip(true)
	lair.Entries = []RoomInitiativeEntry{{ID: testTurnEntryA, Name: "Lair action", Kind: EntryNamed}}
	if strings.Contains(html(t, RoomInitiative(lair)), "data-side") {
		t.Error("a line with no creature was put on a side")
	}
}
func TestOnlyTheActingCardWearsItsNamePlate(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))
	if n := strings.Count(markup, "data-entry-plate"); n != 1 {
		t.Errorf("%d cards wear a name plate, want one", n)
	}
	if n := strings.Count(markup, "data-entry-name"); n != 2 {
		t.Errorf("%d cards carry their name, want two", n)
	}
}
func TestEveryCardIsTheSameSize(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))
	if n := strings.Count(markup, initiativeTile); n != 2 {
		t.Errorf("%d cards are drawn at the one size, want two", n)
	}
}
func TestTheCardsFloatWithNothingBehindThem(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))
	for _, panel := range []string{"bg-panel", "backdrop-blur", "border-base-300"} {
		if strings.Contains(markup, panel) {
			t.Errorf("the row is drawn on a panel (%s):\n%s", panel, markup)
		}
	}
}
func TestTheHitPointTextIsTheGMsEverywhereAndThePlayersOnce(t *testing.T) {
	gm := turnStrip(true)
	if !gm.ShowHP(gm.Entries[0]) || !gm.ShowHP(gm.Entries[1]) {
		t.Error("the GM is not shown hit points on every line")
	}
	player := turnStrip(false)
	if !player.ShowHP(player.Entries[0]) {
		t.Error("a player is not shown hit points on the acting line")
	}
	if player.ShowHP(player.Entries[1]) {
		t.Error("a player is shown hit points on a line that is not acting")
	}
	quiet := turnStrip(true)
	quiet.Entries[0].HP = ""
	if quiet.ShowHP(quiet.Entries[0]) {
		t.Error("a line with no hit-point text prints one anyway")
	}
}
func TestTheHiddenBadgeIsTheGMs(t *testing.T) {
	if markup := html(t, RoomInitiative(turnStrip(true))); !strings.Contains(markup, "Hidden") {
		t.Errorf("the GM has no marker for a line players cannot see:\n%s", markup)
	}
	if markup := html(t, RoomInitiative(turnStrip(false))); strings.Contains(markup, "Hidden") {
		t.Error("a player's strip carries the hidden badge")
	}
}
func TestANamedLineHasNoPortraitAndNoPawn(t *testing.T) {
	data := turnStrip(true)
	data.Entries[1] = RoomInitiativeEntry{ID: testTurnEntryB, Name: "Lair action", Kind: EntryNamed}
	markup := html(t, RoomInitiative(data))
	if strings.Contains(markup, "/assets/goblin.webp") {
		t.Error("a named line drew a portrait")
	}
	if !strings.Contains(markup, "Lair action") {
		t.Errorf("the named line lost its name:\n%s", markup)
	}
	if strings.Count(markup, "data-entry-solo") != 1 {
		t.Error("a named line offers a pawn window")
	}
}
func TestOnlyTheActingLineWearsAnAura(t *testing.T) {
	markup := decoded(t, RoomInitiative(turnStrip(true)))
	if n := strings.Count(markup, "data-turn-aura"); n != 1 {
		t.Errorf("%d lines wear an aura, want one:\n%s", n, markup)
	}
	if !strings.Contains(markup, `class="aura aura-silver"`) {
		t.Errorf("the acting line does not carry the component:\n%s", markup)
	}
}
func TestTheAuraCarriesNoReadingOfTheCreature(t *testing.T) {
	markup := decoded(t, RoomInitiative(turnStrip(true)))
	if strings.Contains(markup, `data-turn-aura="`) {
		t.Errorf("the aura carries a value it has nothing to do with:\n%s", markup)
	}
}
func TestTheAuraHoldsTheFrameAndThePlateAndNothingElse(t *testing.T) {
	markup := decoded(t, RoomInitiative(turnStrip(false)))
	aura := strings.Index(markup, "data-turn-aura")
	frame := strings.Index(markup, "data-entry-tile")
	plate := strings.Index(markup, "data-entry-plate")
	next := strings.Index(markup, "data-turn-next")
	if aura < 0 || frame < 0 || plate < 0 || next < 0 {
		t.Fatalf("the acting line is missing a part:\n%s", markup)
	}
	if !(aura < frame && frame < plate && plate < next) {
		t.Errorf("the acting card is not framed, plated and then buttoned:\n%s", markup)
	}
	if n := strings.Count(markup[plate:next], "</span>"); n != 2 {
		t.Errorf("the aura shuts %d tags after the plate, want two:\n%s", n-1, markup)
	}
}
func TestTheStripNamesNoDaisyComponentItDoesNotUse(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true))) +
		html(t, RoomInitiativeEntryForm(RoomInitiativeEntryData{RoomID: testTurnRoomID}))
	if strings.Contains(markup, "card") {
		t.Errorf("the strip says \"card\" somewhere:\n%s", markup)
	}
	banned := map[string]bool{
		"card": true, "list": true, "table": true, "tab": true, "tabs": true,
		"status": true, "stack": true, "swap": true, "indicator": true,
		"steps": true, "timeline": true, "countdown": true, "chat": true,
		"mask": true, "dock": true, "diff": true, "hero": true, "drawer": true,
		"progress": true, "rating": true, "divider": true, "collapse": true,
		"carousel": true, "skeleton": true, "accordion": true, "range": true,
	}
	for _, value := range classValues(markup) {
		for _, name := range strings.Fields(value) {
			if banned[name] {
				t.Errorf("the strip carries the DaisyUI class %q:\n%s", name, markup)
			}
		}
	}
}
func classValues(markup string) []string {
	var out []string
	rest := markup
	for {
		at := strings.Index(rest, `class="`)
		if at < 0 {
			return out
		}
		rest = rest[at+len(`class="`):]
		shut := strings.Index(rest, `"`)
		if shut < 0 {
			return out
		}
		out = append(out, rest[:shut])
		rest = rest[shut:]
	}
}
func TestTheSplatterIsStableAndInRange(t *testing.T) {
	for _, id := range []string{testTurnPawnA, testTurnEntryA, testTurnEntryB, ""} {
		first := InitiativeBloodVariant(id)
		if first != InitiativeBloodVariant(id) {
			t.Errorf("%q chose two different splatters", id)
		}
		if first < "1" || first > "9" {
			t.Errorf("%q chose splatter %q, which is not one of the nine", id, first)
		}
	}
}
func TestTheEntryFieldMatchesTheProtocolsNameLimit(t *testing.T) {
	if EntryNameMax != "128" || room.NameLimit != 128 {
		t.Errorf("the field takes %s characters and the protocol takes %d", EntryNameMax, room.NameLimit)
	}
}
