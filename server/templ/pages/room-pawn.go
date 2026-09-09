package pages

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// ONE PAWN, ONE SURFACE, AND THE SURFACE IS A WINDOW.
//
// THE PANEL IS WHAT A RIGHT CLICK OPENS AND IT STAYS OPEN THROUGH A FIGHT. It
// is the interaction the old client had that was worth carrying forward: a
// goblin's hit points, armour class and conditions, tucked in a corner, several
// at once, refetching themselves as the socket says they changed. Everything
// about the pawn is in it -- its hit points, its size, its armour class, its
// conditions, whether players can see it and which floor it stands on.
//
// THERE WAS AN EDIT MODAL AND IT HAS BEEN TAKEN OUT. It held every field listed
// above, reached by an Edit button on this panel, and each of them is something
// a GM wants beside the value they are watching rather than behind a button
// that blocks the page. Two surfaces for one pawn also meant two error slots,
// two ids for every field, and a Save that dismissed a dialog rather than
// answering with what it had changed.
//
// NOTHING HERE IS SAVED BY PRESSING ANYTHING. There is no Save button, for the
// reason the character sheet has none: a form whose button is below the fold of
// a 320 pixel window is a form that looks broken, and the reader who cannot see
// it concludes the fields do not work. Every control autosaves -- the editor on
// a debounced input, the hit-point boxes on change -- and both forms answer with
// the panel's ERROR SLOT rather than with the panel. That is the whole of why
// autosaving here is safe: a save that swapped the panel would replace the field
// somebody had just tabbed into, which is precisely what an autosave does every
// few seconds. See pawnSaveTrigger.
//
// THE NAME IS RENAMED THROUGH A DIALOG AND NOT THROUGH A FIELD, which is the
// one thing here that is not a control in the panel. A name is changed once in
// a session and read every second of it, so it is a heading with a button
// beside it rather than an input taking up a row for ever; and an autosaving
// text field that renames a pawn on every pause mid-word would rename it four
// times to get to "Goblin archer". A player's character has no such button at
// all -- see RoomPawn.Character.
//
// THE FLOOR SELECT AND VISIBILITY ARE THE FIRST ROW OF THE EDITOR, in that
// order, above the size and the armour class: they are the two controls a GM
// reaches for mid-fight and everything else in the form is set once. They were
// at the bottom, under the condition rows and the Add condition button, which
// on a pawn with three conditions is below the fold of the default window.
//
// THE FLOOR TAKES THE ROW AND THE SWITCH SITS AT ITS END, which is why the
// select carries flex-1 and the switch does not shrink. A floor name is as long
// as the GM made it and a switch is two words at most, so the one that can use
// the space gets it.
//
// THEY ARE NOT IN THE HEADER BESIDE THE BUTTONS, which is where they would sit
// most naturally, and the reason is mechanical: both are form controls that the
// editor's POST has to carry, the editor's trigger listens on its own form, and
// a control outside a form does not raise events into it. The header's three
// buttons each make a request of their own, so they have no such constraint.
//
// EVERY BUTTON IN THE HEADER IS AN ICON WITH A TOOLTIP AND AN ACCESSIBLE NAME.
// The header is one line shared with a portrait, a name and a floor, and a
// worded button on it is a button that pushes the name it belongs to out of
// view at 260 pixels. The tip says what it does on hover; the aria-label says
// which pawn it does it to, because eight goblins are eight of these.
//
// TWO OF THESE ARE OPEN AT ONCE AS A MATTER OF COURSE, which is what makes
// every id in this file carry the pawn's own: the panel's element, its error
// slot, its form, its conditions list, its datalist and every labelled control.
// A shared id is a label that focuses another window's field and a POST whose
// errors land in somebody else's panel.
//
// EVERYTHING HERE HAS ALREADY BEEN PROJECTED. The controller reads the pawn
// through hub.Pawn, which answers with the copy the asking role may see -- so a
// player looking at a monster in a band room is handed a band and no numbers,
// and there is nothing in this file that could reveal one. A field that is
// empty here is empty because the viewer was never told it.

// RoomPawnPanel names the error slot above the hit-point field. It is a prefix
// rather than the whole id: two pawn windows are open at once as a matter of
// course, and two elements sharing an id is a POST whose errors land in
// somebody else's window.
const RoomPawnPanel = "pawn"

// RoomPawnRenamePanel is the same slot for the rename dialog, and it needs no
// pawn id: there is one content modal on the page and it holds one dialog at a
// time, so the two panels that could collide here cannot both exist.
const RoomPawnRenamePanel = "pawn-rename"

// pawnSaveTrigger is how the editor saves: a debounced keystroke, plus the
// event repeater.js raises when a condition row is deleted.
//
// IT IS THE CHARACTER SHEET'S PANEL TRIGGER WITH A SHORTER FUSE. The sheet
// waits a second because its fields are prose that one person is writing; this
// is short numbers and selects, and what they change is on a table half a dozen
// people are looking at -- a floor select that took a second to move a pawn
// upstairs would read as a control that did not work.
//
// THE HIT-POINT BOXES ARE DELIBERATELY NOT ON THIS. They take arithmetic, and a
// sum debounced on input is a different sum at every keystroke: "23-" is not a
// number and "23-7" posted mid-entry is a goblin on 16 before the person had
// finished typing 23-7-4. They fire on change, which is blur or Enter, which is
// when a sum is finished. See js/room/hp.ts.
const pawnSaveTrigger = "input delay:400ms, repeater:changed"

// HPEntryMax is the maxlength of a hit-point box, as the attribute prints it.
// It is not a bound on hit points -- the core decides those -- but on the
// arithmetic the box accepts, and it is js/room/hp.ts's LIMIT written where the
// markup can reach it.
const HPEntryMax = "24"

// ConditionNames is the twenty familiar conditions, offered as a datalist
// rather than enforced as a set.
//
// THE SERVER TAKES ANY NAME AND THAT IS DELIBERATE. A table invents conditions
// -- "on fire", "holding the rope", "marked" -- and a closed vocabulary would
// mean these twenty and nothing else. What the list buys is that the twenty are
// spelled the same way every time, so a search or a sort over them works.
var ConditionNames = []string{
	"Blinded", "Charmed", "Concentrating", "Deafened", "Exhaustion",
	"Frightened", "Grappled", "Incapacitated", "Invisible", "Paralyzed",
	"Petrified", "Poisoned", "Prone", "Restrained", "Stunned",
	"Unconscious", "Blessed", "Hasted", "Slowed", "Raging",
}

// ConditionColors is the eight rings a condition can be drawn in, in the order
// the select offers them.
//
// THEY ARE NAMES AND NOT HEX, because the ring is drawn twice -- once as a chip
// in this panel and once as a circle on the canvas -- and a hex string chosen in
// a form would have to travel to the renderer and be trusted there. Eight names
// map to eight colours in each place, and the protocol validates the name.
var ConditionColors = []string{"red", "orange", "yellow", "green", "blue", "purple", "pink", "white"}

// ObjectPixelsMax is the object size the form allows on one axis, in MAP
// PIXELS, and it is room.ObjectPixelsMax written here rather than imported.
//
// THE PACKAGE IS NOT IMPORTED BY THE MARKUP ON PURPOSE. templ/pages renders
// HTML and has no business holding a protocol type; what it needs is the number
// the input's max attribute prints, and the core refuses anything past it
// whatever this says. A test pins the two together so a change to one is a
// failure rather than a form that accepts what the server will not.
const ObjectPixelsMax = 8_192

// ConditionColorLabel is a colour name as the select prints it.
func ConditionColorLabel(color string) string {
	if color == "" {
		return ""
	}

	return strings.ToUpper(color[:1]) + color[1:]
}

// ConditionClears is what a duration counts down against: the start or the end
// of the pawn's own turn, which is the distinction 5e draws between a condition
// lasting "until the start of your next turn" and one lasting "until the end of
// it".
var ConditionClears = []Option{
	{Label: "End of turn", Value: "end"},
	{Label: "Start of turn", Value: "start"},
}

// conditionNameList ties one pawn's condition fields to that pawn's own
// <datalist> of the twenty familiar names, and conditionNamesID is the id it
// points at.
//
// IT IS AN ATTRIBUTE BUILT IN GO RATHER THAN WRITTEN IN THE MARKUP, and that is
// not a style choice. Tailwind reads every .templ file as TEXT and takes a
// class-name candidate from anything word-shaped in it, attribute names
// included -- so `list="condition-names"` in the template puts DaisyUI's whole
// .list family in the stylesheet: fourteen selectors and about two kilobytes
// for a component this app does not use, with nothing failing anywhere.
//
// The usual answer is to pick a different word, which CLAUDE.md gives for a DOM
// event or a form field. This one is HTML's own attribute and cannot be
// renamed, so it moves to a file the scanner does not read instead. Measured
// with the selector diff, not assumed.
//
// THE PAWN'S ID IS IN IT because two panels are open at once and each brings
// its own list. Two elements with one id is invalid, and every input in both
// windows would resolve to whichever happened to be first in the document --
// which works right up until that window is closed.
func conditionNamesID(pawnID string) string { return "condition-names-" + pawnID }

func conditionNameList(pawnID string) templ.Attributes {
	return templ.Attributes{"list": conditionNamesID(pawnID)}
}

// RoomPawnData is the live panel: one pawn, already projected, plus who is
// looking at it.
type RoomPawnData struct {
	RoomID string

	// CanEdit is the GM or the pawn's owner, and it is what draws the form
	// instead of the readings. It is a courtesy and not the authorization:
	// PawnUpdate.Authorize refuses the same people again on every post.
	CanEdit bool

	// IsGM draws the controls that are the GM's alone -- the hidden marker,
	// the stat block button, the visibility toggle, the floor select and
	// Remove.
	IsGM bool

	Pawn RoomPawn

	// Layers is the room's floors, for the GM's floor select, and LayerID is
	// the one the pawn stands on now.
	//
	// IT IS EMPTY FOR A PLAYER, who may not move a pawn between floors at all:
	// PawnSetLayer refuses them, because a player who sent their own pawn
	// upstairs would stop being sent it and would be holding a pawn they can no
	// longer see.
	Layers  []RoomPawnLayer
	LayerID string

	// Shown is the visibility toggle's state, GM only. It is the opposite of
	// RoomPawn.Hidden and both are here because one is a control's value and
	// the other is a badge in the header.
	Shown bool
}

// RoomPawnRenameData is the rename dialog: one field, prefilled.
type RoomPawnRenameData struct {
	RoomID string
	PawnID string
	Name   string
}

// SavePath is the rename's own route rather than the panel's save.
//
// A ROUTE OF ITS OWN BECAUSE THE EDITOR'S POST IS THE WHOLE FORM. That handler
// replaces a pawn's conditions with the rows the form carried, so a dialog
// posting one field to it would take every condition off the goblin on its way
// past. One field, one route, one command.
func (d RoomPawnRenameData) SavePath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.PawnID + "/name"
}

// RoomPawn is one pawn as the panel draws it: strings, because every number on
// it has already been decided to be shown or withheld and a nil int in a
// template is a decision waiting to be made twice.
type RoomPawn struct {
	ID   string
	Name string

	// Image is the picture the panel shows beside the name, and it is the same
	// URL the canvas draws its sprite from.
	Image string

	// Object switches the whole shape of the panel: a picture's width and
	// height instead of a creature size, and no conditions at all, because a
	// wagon cannot be poisoned.
	Object bool

	// Character is a pawn that IS somebody at the table -- a player's own
	// character -- and the only thing it decides is that there is no Rename
	// button on it.
	//
	// A CHARACTER'S NAME IS NOT THE GM'S TO CHANGE AND IS NOT CHANGED MID-GAME.
	// It came from the sheet its player wrote, every other player reads it in
	// the turn order and the chat, and the one plausible reason to edit it here
	// -- a typo -- is a thing to fix on the sheet, where it will still be right
	// next session. A goblin is the opposite case: "Goblin" becomes "Goblin
	// archer" the moment there are two of them, which is what the button is for.
	Character bool

	// HP is "12 / 20" when the viewer is given numbers, and empty when they are
	// not. Band is the word -- Very bloody -- when a word is all they get.
	// Exactly one of the two is set, and both are empty in a room labelling
	// nothing.
	HP   string
	Band string

	// HPValue and MaxHP are the two editable boxes, which is the same pair HP
	// prints as one string for a reader. They are separate from it because the
	// row is two fields with a slash between them and the reading is a
	// sentence: "4 / 7" cannot be typed into and neither box can be read.
	HPValue string
	MaxHP   string

	// AC is the number, printed as it stands in the panel and used as the
	// form's value. It is empty when the viewer was told nothing.
	AC string

	// Size is the label a reader's panel prints for a CREATURE and SizeValue is
	// what the editor's select is set to. Pixels is the same line for an OBJECT --
	// "200 by 140 pixels, turned 30 degrees" -- and Width, Height and Rotation
	// are the three numbers its form takes. The panel prints whichever of the
	// two applies under one heading, because "how big is it" is one question
	// with two kinds of answer.
	//
	// THE ANGLE IS PART OF THE SIZE LINE RATHER THAN A HEADING OF ITS OWN. It
	// is empty far more often than not -- most tokens are never turned -- and a
	// row reading "Rotation: 0 degrees" under every wagon on the table is a row
	// that says nothing.
	Size      string
	SizeValue string
	Pixels    string
	Width     string
	Height    string
	Rotation  string

	// Layer is the floor's name, which is worth showing because a pawn's panel
	// can outlive the GM's view of the floor it stands on.
	Layer string

	Conditions []RoomPawnCondition

	// Hidden is the GM's marker for a pawn players cannot see. A player never
	// receives such a pawn at all, so this is never true on their copy.
	Hidden bool

	// MonsterID opens the stat block window and is set for the GM alone.
	//
	// PLAYERS DO NOT GET A STAT BLOCK, and that is not timidity. The room's
	// monster-health setting exists so a table can hide a monster's hit points;
	// a stat block carries those, its armour class, its resistances and its
	// legendary actions, so handing one to a player would contradict the
	// setting the GM chose in the same window.
	MonsterID string
}

// RoomPawnCondition is one chip in the panel and one row in the form.
//
// THE ROW IS FIVE CONTROLS AND THEY DO NOT FIT ON ONE LINE IN A 320 PIXEL
// WINDOW. A name, a colour, a number of turns, what it counts down against and
// a remove button need about 260 pixels of fixed width between them before the
// name has anywhere to go, so the row was overflowing its panel sideways and
// the window was answering with a horizontal scrollbar. It stacks now -- the
// name on its own line, the four small controls under it -- and goes back to
// one line at a container width where the name still has room to be read. Every
// control in it can shrink, so the minimum window size clips nothing.
//
// DURATION IS CARRIED TWICE AND THAT IS NOT REDUNDANCY. The form's number input
// takes the protocol's own value, where -1 means "until somebody removes it";
// the chip prints "3 turns left" and nothing at all for -1, because -1 is a
// sentinel and not a fact about the goblin.
type RoomPawnCondition struct {
	ID           string
	Name         string
	Color        string
	Duration     string
	DurationText string
	Clear        string
}

// RoomPawnLayer is one option in the layer select.
type RoomPawnLayer struct {
	ID   string
	Name string
}

// Panel is the error slot the form and the hit-point field both write into.
func (d RoomPawnData) Panel() string { return RoomPawnPanel + "-" + d.Pawn.ID }

// ElementID is the panel's own id, which is also its refetch target and what
// both of its forms swap.
func (d RoomPawnData) ElementID() string { return "pawn-" + d.Pawn.ID }

// FormID is the editor's form element, ConditionsID is the list its Add button
// appends to, and NamesID is the datalist its condition fields read.
func (d RoomPawnData) FormID() string { return "pawn-form-" + d.Pawn.ID }

func (d RoomPawnData) ConditionsID() string { return "pawn-conditions-" + d.Pawn.ID }

func (d RoomPawnData) NamesID() string { return conditionNamesID(d.Pawn.ID) }

// Field is one control's id, and it carries the pawn's own for the reason
// every other id here does: two panels are open at once, and a <label for> that
// named a bare "name" would put the caret in the other window's field.
func (d RoomPawnData) Field(name string) string { return name + "-" + d.Pawn.ID }

// Path is the fragment's own URL, so the copy swapped in refetches itself the
// same way the first one did.
func (d RoomPawnData) Path() string {
	return "/fragment/room/pawn?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}

// RenamePath is the fragment the Rename button opens in the content modal. It
// is a /fragment/ URL because the modal refuses anything else, for the reason a
// window does: a page swapped into a dialog is a whole document inside a panel.
func (d RoomPawnData) RenamePath() string {
	return "/fragment/room/pawn/rename?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}

// Trigger is the refetch, and both halves of it are load-bearing.
//
// THE FILTER ON detail.id IS WHAT MAKES TEN WINDOWS COST ONE REQUEST. panels.ts
// turns pawn.updated into a room:pawn event carrying the id that changed; every
// open panel hears it and all but one decline. Without the filter, one goblin
// taking damage is a GET per open window, every round, for the whole fight.
//
// THE FILTER ON activeElement IS WHAT KEEPS A REFETCH FROM EATING A KEYSTROKE,
// and it asks about a TYPING field rather than about the panel. A swap replaces
// the box somebody is halfway through filling in and loses what was in it --
// but only a box that is being filled in. A select or a checkbox has nothing
// half-entered to lose, and those are exactly the controls whose own save has
// to bring the panel back: ticking "players can see this pawn" changes the
// Hidden badge in the header, and a refetch declined because the checkbox still
// had focus would leave the badge contradicting the box beside it.
//
// hx-sync="this:queue last" is in the markup beside this and is not optional. A
// burst of events is a burst of GETs whose answers can land in either order --
// the socket is ordered, a pair of HTTP requests is not -- and "queue last" runs
// them one at a time keeping only the newest. Never "replace": that cancels the
// request in flight, and htmx reports every cancellation as a console error.
func (d RoomPawnData) Trigger() string {
	return "room:pawn[detail.id === '" + d.Pawn.ID + "' && !" + typingInPanel + "] from:window"
}

// typingInPanel is the half of the filter above that asks whether this panel
// holds the caret, and it asks it by the control's TYPE rather than with a CSS
// selector.
//
// NEITHER A SQUARE BRACKET NOR A COMMA MAY APPEAR IN AN hx-trigger FILTER. The
// filter is delimited by the brackets around it and the whole attribute is split
// on commas, so 'input[type=text],textarea' -- the obvious way to write this --
// ends the filter early and leaves the rest of the expression parsed as trigger
// modifiers. What comes out is not an error; it is a panel that has quietly
// stopped refetching.
//
// A TYPE OF text OR number IS WHAT "TYPING" MEANS HERE, and it is deliberately
// the positive list. Every box in this panel is one or the other; a select
// answers "select-one", a checkbox "checkbox", and anything that is not a form
// control answers undefined -- so the three controls whose own save has to bring
// the panel back are covered by not being in the list, rather than by being
// remembered in a list of exceptions.
const typingInPanel = "(this.contains(document.activeElement) && " +
	"(document.activeElement.type === 'text' || document.activeElement.type === 'number'))"

// HPPath is where the hit-point row posts, both boxes together. It is a
// resource URL and not a fragment because it is a mutation, and it answers with
// the error slot rather than with anything to swap into the row -- the boxes
// resolve their own arithmetic in the browser, so there is nothing left for the
// reply to tell them.
func (d RoomPawnData) HPPath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.Pawn.ID + "/hp"
}

// StatBlockPath, StatBlockWindow and StatBlockTitle are the three attributes
// the stat block trigger carries.
//
// THE WINDOW IS KEYED BY THE MONSTER AND NOT BY THE PAWN, which is what keeps
// eight goblins from opening eight identical windows. Opening it from the
// second goblin brings the first one's window forward, which is right: it is
// the same page of the same book.
func (d RoomPawnData) StatBlockPath() string {
	return "/fragment/room/stat-block?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}

func (d RoomPawnData) StatBlockWindow() string { return "monster:" + d.Pawn.MonsterID }

// SavePath is where the editor posts, and it answers with the panel it just
// changed -- which is the case the fragment rules name for a mutation outside
// /fragment/.
func (d RoomPawnData) SavePath() string {
	return "/rooms/" + d.RoomID + "/pawns/" + d.Pawn.ID
}

// RemovePath is the DELETE behind the confirm modal. The ids travel as form
// values rather than in the path because the same route takes a whole selection
// from the canvas overlay, and one route that takes a list is better than two
// that differ in how many.
func (d RoomPawnData) RemovePath() string {
	return "/rooms/" + d.RoomID + "/pawns"
}

// RemoveLabel is what a screen reader is told the Remove button does, and it is
// built HERE rather than written in the markup for the reason conditionNameList
// is. Tailwind reads every .templ file as text, attribute values included, and
// the word "table" in one of them puts DaisyUI's whole .table family in the
// stylesheet -- three selectors and a kilobyte for a component this window does
// not use, with nothing failing anywhere. Measured with the selector diff.
//
// IT NAMES THE PAWN because the button is an icon in a row of icons, and "remove"
// on its own does not say remove what. A sighted reader has the panel around it
// to answer that; a reader listening to the button has this string and nothing
// else.
func (d RoomPawnData) RemoveLabel() string {
	return "Remove " + d.Pawn.Name + " from the table"
}

// CanRename draws the Rename button, and it is narrower than CanEdit by exactly
// one case: a player's character. See RoomPawn.Character.
func (d RoomPawnData) CanRename() bool {
	return d.CanEdit && !d.Pawn.Character
}

// HasActions is whether the header's button row has anything in it, and it
// exists so that the row is not rendered empty.
//
// AN EMPTY FLEX CHILD IS NOT FREE. The header is a flex row with a gap, and a
// container holding no buttons still takes a gap beside itself -- which is a
// few pixels of nothing between a player's portrait and the right-hand edge, on
// the one panel that has no buttons at all.
func (d RoomPawnData) HasActions() bool {
	return d.IsGM || d.CanRename()
}

// The accessible names for the three icon buttons in the header, each naming
// the pawn it acts on.
//
// THEY ARE LONGER THAN THE TOOLTIPS BESIDE THEM AND THAT IS THE POINT. A tip is
// read next to the thing it points at, so "Remove" is unambiguous; a screen
// reader announces the button with no such context, and eight goblins in a room
// means eight buttons that would all announce as "Remove". They live in Go
// rather than in the markup for the reason every string here does: Tailwind
// reads a .templ file as text and takes a class-name candidate out of ordinary
// prose in an attribute value.
func (d RoomPawnData) StatBlockLabel() string { return "Stat block for " + d.Pawn.Name }

func (d RoomPawnData) RenameLabel() string { return "Rename " + d.Pawn.Name }

func (d RoomPawnData) VisibleLabel() string { return "Players can see " + d.Pawn.Name }

// RemovePrompt names what is about to go, because "Are you sure?" over a table
// of goblins is a question nobody can answer safely.
func (d RoomPawnData) RemovePrompt() string {
	return "Remove " + d.Pawn.Name + " from the table. This cannot be undone."
}

// RemoveVals is the one id the panel's Remove sends, in the shape the route
// takes from the canvas overlay as well. One route that takes a list serves
// both, and the alternative is two routes that differ only in how many pawns
// they name.
func (d RoomPawnData) RemoveVals() string {
	return `{"ids": "` + d.Pawn.ID + `"}`
}

// AddTurnPath, AddTurnVals and AddTurnLabel are the header's Add to initiative
// button, which is the GM's.
//
// IT IS HERE AS WELL AS ON THE RIGHT-CLICK MENU, and the pair is decision 19 of
// the turn-order design rather than two ways to do one thing. Sync tracker
// covers every creature a player can see on a floor a player is standing on,
// which is the fight; what neither Sync nor a checklist reaches is the creature
// the GM is looking at RIGHT NOW that Sync would not take -- one that is hidden,
// or waiting on an empty floor to burst in. This window and that menu are the
// two places a GM is already looking at one pawn.
//
// IT IS NOT DISABLED FOR A PAWN THAT IS ALREADY IN THE ORDER, because this
// panel is not told what is in the order and asking would be a second read on
// every refetch of every open window. The route answers that one with a
// sentence in the alert modal.
func (d RoomPawnData) AddTurnPath() string {
	return "/rooms/" + d.RoomID + "/initiative"
}

func (d RoomPawnData) AddTurnVals() string {
	return `{"pawn": "` + d.Pawn.ID + `"}`
}

func (d RoomPawnData) AddTurnLabel() string {
	return "Add " + d.Pawn.Name + " to the initiative tracker"
}

// ConditionRowPath is where the Add condition button fetches an empty row.
//
// IT IS A FRAGMENT AND NOT A CLIENT-SIDE CLONE, which is the shape the
// character sheet's repeaters already have: repeater.js removes rows and the
// server renders them, so the markup for a row exists once. A row built in
// JavaScript would be a second copy of it, in a file Tailwind does not scan.
func (d RoomPawnData) ConditionRowPath() string {
	return "/fragment/room/condition-row?room=" + d.RoomID + "&pawn=" + d.Pawn.ID
}

// PawnHPText is the pair the panel prints, and it is empty when the viewer was
// given no numbers at all.
func PawnHPText(hp *int, maxHP *int) string {
	if hp == nil {
		return ""
	}
	if maxHP == nil {
		return strconv.Itoa(*hp)
	}

	return strconv.Itoa(*hp) + " / " + strconv.Itoa(*maxHP)
}

// PawnBandText is the word a band projects to. It is title case because it is
// printed as a label rather than read as a value.
//
// IT HAS A TWIN IN js/room/overlay.ts, which prints the same word over the pawn
// on the table while this one prints it in the pawn's window. The two saying
// different things about one goblin is the bug this note exists to make
// findable; TestEveryBandHasAWord and its TypeScript opposite pin the pair.
func PawnBandText(band string) string {
	switch band {
	case "healthy":
		return "Healthy"
	case "bruised":
		return "Bruised"
	case "bloody":
		return "Bloody"
	case "veryBloody":
		return "Very bloody"
	case "nearDeath":
		return "Near death"
	case "dead":
		return "Dead"
	}

	return ""
}

// PawnSizeText is a size category as a label.
func PawnSizeText(size string) string {
	if size == "" {
		return ""
	}

	return strings.ToUpper(size[:1]) + size[1:]
}

// PawnPixelsText is an object's rectangle, in map pixels, and the angle it has
// been turned to. It is the creature size line's opposite number: a wagon has
// no size category to print, so what it prints instead is how large the picture
// on the table is and which way round it is lying.
//
// A SQUARE TOKEN SAYS NOTHING ABOUT ITS ANGLE, because zero degrees is the
// absence of a rotation rather than a fact about the wagon. Most tokens are
// never turned and the clause would be noise on every one of them.
func PawnPixelsText(w int, h int, rotation int) string {
	if w <= 0 || h <= 0 {
		return ""
	}

	out := strconv.Itoa(w) + " by " + strconv.Itoa(h) + " pixels"
	if rotation != 0 {
		out += ", turned " + strconv.Itoa(rotation) + " degrees"
	}

	return out
}

// PawnDurationText is a condition's remaining turns, and -1 is not a number
// anybody wants to read. It is the value the protocol gives "until somebody
// takes it off", which is most conditions most of the time.
func PawnDurationText(turns int) string {
	if turns < 0 {
		return ""
	}

	// ONE TURN IS NOT "1 turns left". The chip is read at a glance by somebody
	// deciding whether to spend a spell slot on it, and the last turn of a
	// condition is the one that gets read most.
	if turns == 1 {
		return "1 turn left"
	}

	return strconv.Itoa(turns) + " turns left"
}

// PawnDurationValue is the same field as the form's number input takes, where
// -1 IS the value and an empty box would mean something else.
func PawnDurationValue(turns int) string { return strconv.Itoa(turns) }
