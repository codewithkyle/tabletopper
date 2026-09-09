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

// turnStrip is a fight in progress: a player's own creature acting, and a
// goblin waiting.
func turnStrip(isGM bool) RoomInitiativeData {
	return RoomInitiativeData{
		RoomID: testTurnRoomID,
		IsGM:   isGM,
		Entries: []RoomInitiativeEntry{
			{
				ID: testTurnEntryA, Name: "Ari", Kind: EntrySolo,
				Image: "/assets/ari.webp", Band: "bloody", Blood: "4",
				HP: "6 / 14", Active: true, Mine: !isGM, Solo: testTurnPawnA,
				Conditions: []RoomPawnCondition{
					{ID: "01COND", Name: "Blessed", Color: "yellow", DurationText: "2 turns"},
				},
			},
			{
				ID: testTurnEntryB, Name: "Goblin", Kind: EntrySolo,
				Image: "/assets/goblin.webp", Band: "dead", Blood: "7",
				HP: "0 / 7", Hidden: isGM, Solo: "01BX5ZZKBKACTAV9WEVGEMMVW4",
			},
		},
	}
}

// THE REFETCH IS THE WHOLE OF HOW THIS SURFACE STAYS LIVE, so both halves of it
// are pinned: the event it listens for, the filter that keeps a refetch from
// dropping a line somebody is dragging, and the queue that keeps a burst of
// them from landing out of order.
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

	// A ROOT THAT ALSO SAID load WOULD FETCH ITSELF AGAIN ON EVERY SWAP, for
	// ever. Only the placeholder the page renders carries it.
	if strings.Contains(markup, `hx-trigger="load`) {
		t.Error("the fragment's own root carries load")
	}
	if !strings.HasPrefix(InitiativeLoadTrigger, "load, ") {
		t.Error("the placeholder's trigger does not fetch once on load")
	}
}

// NEITHER A SQUARE BRACKET NOR A COMMA MAY APPEAR INSIDE AN hx-trigger FILTER.
// The filter is delimited by the brackets around it and the attribute is split
// on commas, so either one ends the expression early and leaves the rest parsed
// as trigger modifiers -- which is not an error, it is a strip that has quietly
// stopped refetching.
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

// AN EMPTY TRACKER IS NO STRIP AT ALL, not an empty panel: the table underneath
// it is what the room is for.
func TestAnEmptyTrackerRendersHidden(t *testing.T) {
	markup := html(t, RoomInitiative(RoomInitiativeData{RoomID: testTurnRoomID, Empty: true}))

	if !strings.Contains(markup, "hidden") {
		t.Errorf("an empty tracker is not hidden:\n%s", markup)
	}
}

// THE BAND IS THE ONE ATTRIBUTE EVERY WOUND HANGS OFF. The stylesheet spends it
// on blood, pallor, a pulse and a skull; the markup's whole job is to render it.
func TestEveryFaceCarriesItsBand(t *testing.T) {
	markup := html(t, RoomInitiative(turnStrip(true)))

	for _, want := range []string{`data-band="bloody"`, `data-band="dead"`, `data-blood="4"`, `data-blood="7"`} {
		if !strings.Contains(markup, want) {
			t.Errorf("the strip is missing %s:\n%s", want, markup)
		}
	}

	// A creature nobody told this viewer anything about carries no attribute
	// at all and is drawn plain.
	plain := turnStrip(true)
	plain.Entries[0].Band = ""
	plain.Entries[0].Blood = ""

	first := strings.SplitN(html(t, RoomInitiative(plain)), testTurnEntryB, 2)[0]
	if strings.Contains(first, "data-band") {
		t.Errorf("a creature with no band was given one:\n%s", first)
	}
}

// A GROUP IS A DOT PER MEMBER IN THAT MEMBER'S OWN COLOUR, and past twelve it
// is a count -- twenty four-pixel discs under a portrait is a texture rather
// than a reading.
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

	// A GROUP CARRIES NO HIT-POINT TEXT AND NO CONDITION CHIPS. Nine goblins
	// have nine of each, and the rings on the table are where that lives.
	data.Entries[1].HP = ""
	data.Entries[1].Conditions = nil
}

// THE TIMER AND End turn ARE THE ACTIVE PLAYER'S AND NOBODY ELSE'S. A clock the
// whole table can read is a stopwatch on whoever is thinking.
func TestTheTimerAndEndTurnBelongToTheActingPlayerAlone(t *testing.T) {
	player := html(t, RoomInitiative(turnStrip(false)))

	if !strings.Contains(player, "data-turn-timer") || !strings.Contains(player, "End turn") {
		t.Errorf("the acting player was given no clock:\n%s", player)
	}

	// THE KEY PRESSES THE BUTTON THAT IS ALREADY ON THE SCREEN, so the
	// player's End turn and the GM's hidden one carry the same hook and the
	// client learns no route.
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

	// AND THE GM'S IS NEVER SEEN. A Next button used to sit at the far end of
	// the row; it was a control inside a display and it moved every time the
	// order changed. What is left is the element the key presses.
	if !strings.Contains(gm, "data-turn-next hidden") {
		t.Errorf("the GM's turn button is drawn on the strip:\n%s", gm)
	}
}

// THE CONTROLS ARE THE GM'S. What a player gets on this surface is the reading
// and the one button that ends their own turn.
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

// THE HIT-POINT TEXT IS THE GM'S ON EVERY LINE AND A PLAYER'S ON THE ACTING
// LINE ALONE. On the other eleven it would be a column of numbers under a row
// of faces, which is the spreadsheet this design exists to not be.
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

	// A ROOM LABELLING NOTHING PRINTS NOTHING, which is the projection's
	// decision arriving here as an empty string.
	quiet := turnStrip(true)
	quiet.Entries[0].HP = ""
	if quiet.ShowHP(quiet.Entries[0]) {
		t.Error("a line with no hit-point text prints one anyway")
	}
}

// THE Hidden BADGE IS THE GM'S MARKER, and a player never receives such a line
// at all -- so this is belt and braces on top of the projection.
func TestTheHiddenBadgeIsTheGMs(t *testing.T) {
	if markup := html(t, RoomInitiative(turnStrip(true))); !strings.Contains(markup, "Hidden") {
		t.Errorf("the GM has no marker for a line players cannot see:\n%s", markup)
	}
	if markup := html(t, RoomInitiative(turnStrip(false))); strings.Contains(markup, "Hidden") {
		t.Error("a player's strip carries the hidden badge")
	}
}

// A LINE WITH NO CREATURE TAKES ITS OWN NAME IN THE DISC'S PLACE, so the
// strip's rhythm survives -- and it offers no pawn window, because there is no
// pawn.
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

// THE WORD "CARD" APPEARS NOWHERE IN THE RENDERED MARKUP, and it is the trap
// this feature walks straight into -- a card is what everybody calls these.
// `card` is a DaisyUI component and Tailwind reads a .templ file as text, so
// the word in a class, an attribute or a Go identifier inside the template
// would put its whole family in the stylesheet with nothing anywhere failing.
// In the markup a line of the tracker is an ENTRY.
//
// THE OTHER NAMES ARE CHECKED AS CLASSES RATHER THAN AS SUBSTRINGS, because
// several of them are inside attributes this markup legitimately carries:
// hx-swap and hx-status:422 are htmx's, and the extractor splits a candidate on
// ":" and "." rather than on "-", so neither of those is the component word.
// What would be is a class.
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

// classValues is every class attribute in a piece of rendered markup.
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

// THE ROUND COUNTER READS AS A DASH BEFORE THE FIGHT STARTS, because zero is a
// round nobody is in.
func TestTheRoundCounterReadsAsADashBeforeTheFirstTurn(t *testing.T) {
	if got := InitiativeRoundText(0); got != "--" {
		t.Errorf("round 0 prints %q", got)
	}
	if got := InitiativeRoundText(3); got != "3" {
		t.Errorf("round 3 prints %q", got)
	}
}

// EVERYBODY AT THE TABLE SEES THE SAME SPLATTER ON THE SAME GOBLIN, which is
// what makes the choice a function of the pawn's id rather than of anything
// local.
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

// THE NAME FIELD ACCEPTS WHAT THE SERVER ACCEPTS. A field that took more than
// the protocol does is a dialog that fills in, posts, and comes back with an
// error nobody could have avoided.
func TestTheEntryFieldMatchesTheProtocolsNameLimit(t *testing.T) {
	if EntryNameMax != "128" || room.NameLimit != 128 {
		t.Errorf("the field takes %s characters and the protocol takes %d", EntryNameMax, room.NameLimit)
	}
}
