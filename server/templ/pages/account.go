package pages

// The account settings dialog: the account's display name, four pickers over
// the values in internal/prefs, and two toggles.
//
// IT IS A DIALOG AND NOT A PAGE because there is nothing else on it. A field,
// four selects, two switches and a Save is a question, and a question belongs in
// the content modal beside "New Character" and "Share this entry" rather than in
// a route with a heading, a layout and a way back.
//
// AND BECAUSE A DIALOG CAN BE OPENED OVER SOMETHING. Two of the settings on it
// govern a tabletop that is running -- whether the camera follows the turn, and
// whether the floor takes blood -- and the moment somebody wants either changed
// is the middle of the fight that made them want it. A page would have meant
// leaving the table to say so. See helpMenu in room.go, which is the second
// place this opens from, and announceSettings in internal/controllers, which is
// what makes the answer land on the table without a reload.

// accountSettingsID is the element the dialog renders into, and the target its
// own form swaps. It is unexported for the reason journalShareID is: the id
// exists so the fragment can replace itself, and nothing outside this package
// has a reason to name it.
const accountSettingsID = "account-settings"

// AccountSettingsPanel names the error block, and is exported because the
// handler builds the same block on a rejected save.
const AccountSettingsPanel = "account-settings"

// AccountSettingsPath is where the dialog is fetched from, and it is a constant
// because two places open it now: the gear at the bottom of the homepage and
// Settings in the room's Help menu. A URL spelled in two files is a URL that
// gets moved in one of them.
const AccountSettingsPath = "/fragment/account/settings"

// The welcome dialog: the same six controls behind a different message, a
// different pair of buttons and a different route.
//
// IT IS A SECOND WRAPPER AND NOT A SECOND COPY. accountSettingsFields is the
// fields, and both dialogs call it -- a fragment is never a second copy of
// markup, and six controls that drifted apart would be six places for the
// stored value to stop being the one on screen.
//
// What differs is everything around them: this one explains why it is asking,
// its save also stamps the account as set up, and its Close says "Not now" and
// posts, so a reader who does not care can end the asking deliberately.
// Escape still closes without posting, which means "ask me again" -- that is
// the whole dismissal design, and it is why the stamp lives in a column rather
// than in the URL that opened this.
const (
	accountWelcomeID    = "account-welcome"
	AccountWelcomePanel = "account-welcome"
)

// DisplayNameLimit mirrors users.username, which is VARCHAR(128). The field
// carries the same number as a maxlength; the handler is what refuses a client
// that ignored it, and refusing there is what keeps MySQL from truncating a
// name rather than rejecting it.
const DisplayNameLimit = 128

// AccountName, in account.templ, is the greeting on the homepage, and it is
// rendered once by the page and never again.
//
// IT WAS SWAPPED OUT OF BAND BY THE SAVE AND IS NOT ANY MORE. That worked
// exactly as long as the homepage was the only place either dialog could be
// opened from, because htmx reports an out-of-band target it cannot find as an
// error -- and the room's Help menu now opens the settings dialog too. What
// arrives instead is an event carrying the new name, which a page with no
// greeting is allowed to ignore; the listener is public/js/account-name.js,
// loaded by the homepage alone because the homepage alone has the element.
//
// THE ID IS THEREFORE FOR THAT SCRIPT AND NOT FOR HTMX, which is the only thing
// about this component that changed shape.

// ZoneGroup is one <optgroup> of the time zone picker, and ZoneOption is one
// city in it.
//
// ONE PICKER NEEDS GROUPING AND THE OTHER THREE DO NOT. Theme, date format and
// clock offer three, five and two entries; zones offer eighty, and eighty flat
// entries is a list nobody scans. Grouping them by region means a reader finds
// their continent first and their city inside it, which is the only affordance
// a plain <select> has to give.
//
// IT ALSO NEEDS A THIRD FIELD, which is why these are not the plain Option the
// other three use. Alias is the zone's older IANA spelling, rendered onto the
// <option> so the welcome dialog's browser detection matches either -- several
// zones still come back from Intl under the name they had before they were
// renamed. Nothing is ever stored under an alias; it exists only so the right
// option gets selected.
type ZoneGroup struct {
	Label string
	Zones []ZoneOption
}

type ZoneOption struct {
	Value string
	Label string
	Alias string
}

// AccountSettingsData is the dialog: the stored display name, plus each picker's
// options and the value currently stored for it.
//
// EVERY LABEL IS BUILT BY THE CONTROLLER, which is this package's rule
// everywhere and earns its keep here. The date and clock options are labelled
// with the current moment rendered in each of them, so the reader chooses by
// reading the answer rather than by decoding "DD/MM/YYYY" -- and the two slash
// formats, which are the same five characters in a different order, are told
// apart by the only thing that tells them apart.
//
// THOSE EXAMPLES ARE RENDERED IN THE SAVED ZONE, not in whatever the zone
// select is showing. The dialog does no client-side work at all, so nothing
// re-renders when a picker changes; a reader switching continent and format
// together sees the combination the next time they open it. The alternative is
// a script that duplicates prefs' layout table in JavaScript to keep a preview
// live, which is a second implementation of the formatting to keep in step with
// the first.
// STORAGE IS THE ONE READ-ONLY THING ON THE DIALOG, and it is here rather than
// on a page of its own because there is nowhere else it belongs: it is a fact
// about the account, and this is the account's dialog. It is also the only
// field on this struct the session could not supply -- see
// AccountSettingsFragment for what that costs.
//
// IT IS A FORMATTED STRING AND NOT A COUNT OF BYTES, like every other label
// this package renders. The controller does the arithmetic and the unit; a
// template that divided by 1024 would be a second place the unit is decided.
//
// The welcome dialog leaves it empty, and the dialog renders no line when it
// is. It shares the pickers and not the rest of this, and an account being
// welcomed has uploaded nothing.
type AccountSettingsData struct {
	// Name is the account's display name: what the homepage greets and what
	// the player list writes in brackets beside a character.
	//
	// IT IS THE ONE FIELD ON THIS DIALOG THE READER TYPES. Everything else is
	// a picker over a fixed list, which is why every other rejection here names
	// a field rather than quoting a value back -- this one is the exception,
	// and the two messages it can produce are in accountDisplayName.
	Name string

	Themes []Option
	Theme  string

	Zones []ZoneGroup
	Zone  string

	DateFormats []Option
	DateFormat  string

	TimeFormats []Option
	TimeFormat  string

	// FollowTurn is the first of the two controls on this dialog that are not
	// about how a page is rendered: it moves the reader's camera onto whoever
	// is acting when the turn moves at a table.
	//
	// IT IS A TOGGLE AND NOT A PICKER, so it has no list of options beside it --
	// which is why it and ShowBlood are bare bools where their four neighbours
	// are a value and the set it came from. There is no second member to offer.
	//
	// THE TWO OF THEM SHARE ONE LEGEND, AND IT IS "Tabletop". They had a
	// heading each -- "Combat camera" and "Blood" -- which described what each
	// switch does and told a reader nothing about how they differ from the four
	// pickers above them. One heading does: everything under it is about the
	// table this account is sitting at rather than about how a page is drawn,
	// which is the only distinction on this dialog worth a heading at all. Two
	// legends over one switch each was also two headings' worth of chrome for
	// two lines of content. See tabletopFields.
	//
	// IT IS ON THE WELCOME DIALOG TOO, and that is not decoration. An unticked
	// checkbox posts nothing at all, so a welcome form that left this control
	// out would post nothing for it, and the save behind both dialogs would read
	// that silence as "off" -- turning a setting that is on by default off for
	// every account, on the dialog that exists to welcome them. See
	// accountSettingsFields, which is the one copy of the controls both dialogs
	// render.
	FollowTurn bool

	// ShowBlood is the other one: whether this reader's tabletop marks the
	// floor when a creature is hit.
	//
	// IT IS A TOGGLE FOR THE SAME REASON AND ON BOTH DIALOGS FOR THE SAME
	// REASON, and the note above FollowTurn is the whole of both arguments.
	//
	// THE DIALOG DOES NOT SAY IT IS PRIVATE AND DOES NOT NEED TO. Turning it
	// off changes nothing for anybody else at the table -- every mark is drawn
	// by one browser out of hit points it watched change, so there is no
	// splatter on the wire to suppress and nothing on the server that would
	// know -- but that is true of every control on this dialog. It is the
	// ACCOUNT's settings; a line explaining that one of them is the reader's
	// own would imply the others might not be. See ShowBlood in internal/prefs
	// for where the guarantee is actually written down.
	ShowBlood bool

	Storage string
}
