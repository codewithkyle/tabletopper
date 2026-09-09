package pages

// The account settings dialog: the account's display name, four pickers over
// the values in internal/prefs, and one toggle.
//
// IT IS A DIALOG AND NOT A PAGE because there is nothing else on it. A field,
// four selects and a Save is a question, and a question belongs in the content
// modal beside "New Character" and "Share this entry" rather than in a route
// with a heading, a layout and a way back.

// accountSettingsID is the element the dialog renders into, and the target its
// own form swaps. It is unexported for the reason journalShareID is: the id
// exists so the fragment can replace itself, and nothing outside this package
// has a reason to name it.
const accountSettingsID = "account-settings"

// AccountSettingsPanel names the error block, and is exported because the
// handler builds the same block on a rejected save.
const AccountSettingsPanel = "account-settings"

// The welcome dialog: the same four pickers behind a different message, a
// different pair of buttons and a different route.
//
// IT IS A SECOND WRAPPER AND NOT A SECOND COPY. accountSettingsFields is the
// fields, and both dialogs call it -- a fragment is never a second copy of
// markup, and five controls that drifted apart would be five places for the
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

// AccountName, in account.templ, is the greeting on the homepage and the same
// element the two settings saves hand back out of band, so a rename lands on
// the page the dialog is open over.
//
// oob IS FALSE ON THE PAGE AND TRUE IN THE REPLY, and there is one component
// rather than two because a fragment is never a second copy of markup -- two
// spans that drifted apart would be a greeting that changed font when it was
// renamed.
//
// THE ONLY PLACE THAT OPENS EITHER DIALOG IS THE HOMEPAGE, which is what makes
// the out-of-band swap safe: htmx reports an oob target it cannot find as an
// error, so a caller from another page would write one to the console on every
// save. If one is ever added, this element goes with it.

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

	// FollowTurn is the one control on this dialog that is not about how a page
	// is rendered: it moves the reader's camera onto whoever is acting when the
	// turn moves at a table.
	//
	// IT IS A TOGGLE AND NOT A PICKER, so it has no list of options beside it --
	// which is why it is a bare bool where its four neighbours are a value and
	// the set it came from. There is no second member to offer.
	//
	// IT IS ON THE WELCOME DIALOG TOO, and that is not decoration. An unticked
	// checkbox posts nothing at all, so a welcome form that left this control
	// out would post nothing for it, and the save behind both dialogs would read
	// that silence as "off" -- turning a setting that is on by default off for
	// every account, on the dialog that exists to welcome them. See
	// accountSettingsFields, which is the one copy of the controls both dialogs
	// render.
	FollowTurn bool

	Storage string
}
