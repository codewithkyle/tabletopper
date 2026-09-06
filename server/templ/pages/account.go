package pages

// The account settings dialog: four pickers over the values in internal/prefs.
//
// IT IS A DIALOG AND NOT A PAGE because there is nothing else on it. Four
// selects and a Save is a question, and a question belongs in the content modal
// beside "New Character" and "Share this entry" rather than in a route with a
// heading, a layout and a way back.

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
// pickers, and both dialogs call it -- a fragment is never a second copy of
// markup, and four selects that drifted apart would be four places for the
// stored value to stop being the selected one.
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

// AccountSettingsData is the dialog: each picker's options, and the value
// currently stored for it.
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
type AccountSettingsData struct {
	Themes []Option
	Theme  string

	Zones []ZoneGroup
	Zone  string

	DateFormats []Option
	DateFormat  string

	TimeFormats []Option
	TimeFormat  string
}
