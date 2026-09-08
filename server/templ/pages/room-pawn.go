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
// about the pawn is now in it -- its name, its size, its maximum hit points,
// its armour class, its conditions, whether players can see it and which floor
// it stands on.
//
// THERE WAS A MODAL AND IT HAS BEEN TAKEN OUT. It held exactly the fields
// listed above, reached by an Edit button on this panel, and every one of them
// is something a GM wants beside the value they are watching rather than behind
// a button that blocks the page. Two surfaces for one pawn also meant two error
// slots, two ids for every field, and a Save that dismissed a dialog rather
// than answering with what it had changed.
//
// THE OBJECTION TO A FORM IN A REFETCHING PANEL IS ANSWERED BY THE PANEL'S OWN
// TRIGGER. A swap while somebody is typing throws away what they typed, which
// is why the form used to be somewhere that never refetches -- but this panel
// already declined every refetch while focus was inside it, to protect the one
// field it had. That guard now covers the whole form. It is the same rule doing
// more work rather than a new rule. See RoomPawnData.Trigger.
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

	// Errors is what a refused field puts above the form. It arrives on a 422,
	// which the form's hx-status:422 lets through the page's noSwap list, and
	// it replaces the error slot rather than the panel.
	Errors []string
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

	// HP is "12 / 20" when the viewer is given numbers, and empty when they are
	// not. Band is the word -- Bloodied -- when a band is all they get. Exactly
	// one of the two is set, and both are empty when the room hides monster
	// health entirely.
	HP   string
	Band string

	// HPValue is what the editable field starts with, which is the current
	// number and not the pair; MaxHP is the "/ 20" printed beside it. They are
	// separate from HP because the editable form prints its two halves either
	// side of an input and the read-only line prints them as one string.
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

// Trigger is the refetch, and both halves of it are load-bearing.
//
// THE FILTER ON detail.id IS WHAT MAKES TEN WINDOWS COST ONE REQUEST. panels.ts
// turns pawn.updated into a room:pawn event carrying the id that changed; every
// open panel hears it and all but one decline. Without the filter, one goblin
// taking damage is a GET per open window, every round, for the whole fight.
//
// THE FILTER ON activeElement IS WHAT KEEPS A REFETCH FROM EATING A KEYSTROKE.
// The panel has one editable field, and a swap while somebody is typing into it
// would replace the field and lose what was in it. Skipping the refetch
// entirely while focus is inside the panel is exact: the POST that follows
// their own entry answers with the panel, which brings it back in step.
//
// hx-sync="this:queue last" is in the markup beside this and is not optional. A
// burst of events is a burst of GETs whose answers can land in either order --
// the socket is ordered, a pair of HTTP requests is not -- and "queue last" runs
// them one at a time keeping only the newest. Never "replace": that cancels the
// request in flight, and htmx reports every cancellation as a console error.
func (d RoomPawnData) Trigger() string {
	return "room:pawn[detail.id === '" + d.Pawn.ID + "' && !this.contains(document.activeElement)] from:window"
}

// HPPath is where the one inline control posts. It is a resource URL and not a
// fragment, because it is a mutation -- and it answers with the panel it just
// changed, which is the case the fragment rules name.
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
func PawnBandText(band string) string {
	switch band {
	case "healthy":
		return "Healthy"
	case "bloodied":
		return "Bloodied"
	case "critical":
		return "Critical"
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

	return strconv.Itoa(turns) + " turns left"
}

// PawnDurationValue is the same field as the form's number input takes, where
// -1 IS the value and an empty box would mean something else.
func PawnDurationValue(turns int) string { return strconv.Itoa(turns) }
