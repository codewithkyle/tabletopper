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

// OptionGroup is a labelled run of options inside one select.
//
// ONE PICKER NEEDS THIS AND THE OTHER THREE DO NOT. Theme, date format and
// clock offer three, five and two entries; zones offer about fifty, and fifty
// flat entries is a list nobody scans. Grouping them by region means a reader
// finds their continent first and their city inside it, which is the only
// affordance a plain <select> has to give.
type OptionGroup struct {
	Label   string
	Options []Option
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

	Zones []OptionGroup
	Zone  string

	DateFormats []Option
	DateFormat  string

	TimeFormats []Option
	TimeFormat  string
}
