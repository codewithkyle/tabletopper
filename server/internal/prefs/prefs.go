// Package prefs is the account settings a reader chooses once and every page
// obeys: the theme it paints in, the zone, date order and clock its timestamps
// are written in, whether the camera at a table goes to whoever is acting,
// whether that table draws blood, and how loud it is. It holds the tokens the
// database stores, the layouts they mean, and nothing that talks to a database
// or an HTTP request.
//
// THE LAST THREE ARE NOT RENDERINGS AND THEY BELONG HERE ANYWAY. Four of these
// settings decide what a page looks like and three decide what a canvas does,
// but all seven are one account's answer to "how do I like this", all seven are
// read off the same join on every request, and splitting them by what they
// happen to affect would be two packages with one shape.
//
// WHAT IS STORED IS INTENT AND WHAT IS RETURNED IS A RENDERING, and the gap
// between the two is the whole point of the package. The column says "dark",
// not "coffee", and "dmy_slash", not "02/01/2006" -- so a DaisyUI theme rename
// or a change of mind about zero-padding is an edit here rather than a
// migration. The constants below match the ENUM members in the users table
// exactly, which is the one place the two spellings have to agree.
package prefs

import (
	"strconv"
	"time"

	// The zone database, compiled into the binary.
	//
	// THE PRODUCTION IMAGE IS alpine:latest, WHICH SHIPS NO /usr/share/zoneinfo.
	// Without this, every LoadLocation below fails there with "unknown time
	// zone" while working perfectly on a developer machine that has the system
	// files -- so the failure would arrive as wrong dates in production and
	// green tests everywhere else.
	//
	// It costs about 450 KB in the binary and buys a self-contained one: the
	// base image can change, and dev, test and production all resolve zones
	// from the same table. The alternative, `apk add tzdata` in the Dockerfile,
	// fixes the image and leaves `go test ./internal/prefs` on any machine
	// without the system files still failing.
	//
	// It is imported HERE and not in main deliberately, against the usual
	// advice, because this is the package that cannot work without it. In main
	// it would be a line nobody connects to this file, and removing it would
	// break the deployed app while every test still passed.
	_ "time/tzdata"
)

// Theme is which palette the page paints in. The values are not DaisyUI theme
// names -- see the mapping in templ/layouts -- because this app has renamed one
// of those already.
type Theme string

const (
	// ThemeSystem renders no data-theme attribute at all, so the page follows
	// prefers-color-scheme. That is a real choice rather than the absence of
	// one: it is the only setting that tracks the OS switching at dusk.
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

// DateFormat is the order and punctuation of a date.
//
// The two textual members exist because they are the only orderings that read
// the same to everyone, and one of them is the default: a reader who never
// opens the settings dialog should not be shown a date whose meaning depends on
// where they live. The slash pair is ambiguous by construction, which is fine,
// because choosing one is what the setting is for.
type DateFormat string

const (
	DateDMYText  DateFormat = "dmy_text"
	DateMDYText  DateFormat = "mdy_text"
	DateMDYSlash DateFormat = "mdy_slash"
	DateDMYSlash DateFormat = "dmy_slash"
	DateISO      DateFormat = "iso"
)

// TimeFormat is the clock.
type TimeFormat string

const (
	Time12H TimeFormat = "12h"
	Time24H TimeFormat = "24h"
)

// Palette is the DaisyUI theme name this setting paints in, and the empty
// string for system.
//
// THE TWO VOCABULARIES ARE SEPARATE ON PURPOSE. The column holds light and
// dark; caramellatte and coffee are the two themes css/app.css configures, and
// the light one has been renamed once already -- it was silk. Storing the
// palette name would have made that rename a migration.
//
// AN EMPTY STRING IS NOT A MISSING ANSWER, it is the answer for system: the
// page renders no data-theme attribute at all, so DaisyUI's prefers-color-
// scheme block applies and the palette follows the OS live. See the shell in
// templ/layouts, which is where that lands in markup.
func (t Theme) Palette() string {
	switch t {
	case ThemeLight:
		return "caramellatte"
	case ThemeDark:
		return "coffee"
	default:
		return ""
	}
}

// DefaultTimezone is where a user is assumed to be until they say otherwise.
// It is a guess, and a wrong one for most of the world; a signup flow that asks
// is what replaces it.
const DefaultTimezone = "America/New_York"

// Default is the reader the app knows nothing about: a signed-out visitor, and
// the zero value of a session. Every parse below falls back to one of these
// fields rather than to an empty string, so a Preferences that came from
// nowhere still renders a real date.
var Default = Preferences{
	Theme:      ThemeSystem,
	Timezone:   DefaultTimezone,
	DateFormat: DateDMYText,
	TimeFormat: Time12H,
	FollowTurn: true,
	ShowBlood:  true,
	PingVolume: PingVolumeMax,
}

// PingVolumeMax is full volume and PingVolumeStep is how coarsely the slider
// moves, both as percentages of the sound's own level.
//
// THE BOTTOM OF THE RANGE IS THE MUTE, which is why there is no separate switch
// beside it. "How loud" and "at all" are one question with one answer, and a dial
// whose left stop is silence says so without a second control that can disagree
// with it.
//
// TEN STEPS RATHER THAN A HUNDRED. A percentage of a sound that is already quiet
// does not have a hundred distinguishable positions, and eleven stops means the
// mute is a stop somebody lands on rather than a pixel they have to find.
const (
	PingVolumeMax  = 100
	PingVolumeStep = 10
)

// Preferences is one reader's seven settings. It is a value, copied freely, and
// carries no location pointer: Location resolves through the zone table, which
// is already a map of loaded locations, so caching one here would only add a
// field the zero value has to lie about.
type Preferences struct {
	Theme      Theme
	Timezone   string
	DateFormat DateFormat
	TimeFormat TimeFormat

	// FollowTurn moves this reader's camera onto whoever is acting when the
	// turn moves, and zooms out far enough to hold a whole group when the line
	// that came up is one.
	//
	// IT IS THE ACCOUNT'S AND NOT THE ROOM'S. Everything else about a fight is
	// the table's -- who is in the order, what they rolled, whose turn it is --
	// and this is not: it is about where one person's viewport points, and a GM
	// running the fight from an overview and a player who wants to be shown
	// their own turn are both right at the same table.
	//
	// THE ZERO VALUE IS THE WRONG ANSWER, which is the one thing to watch for.
	// It is on for everybody by default, so a Preferences built by hand rather
	// than through New or Default has it off -- see New, which is the reason
	// there is only one way to build one of these.
	FollowTurn bool

	// ShowBlood is whether this reader's tabletop marks the floor when
	// somebody is hit. It is the second setting here that is about a canvas
	// rather than a page, and it is the account's for the same reason
	// FollowTurn is: what one person can stomach looking at for four hours is
	// not the table's decision, and a GM running a knife fight has no business
	// deciding it for the player beside them.
	//
	// NOTHING ON THE SERVER OBEYS IT, and there is nothing there that could.
	// Every mark is drawn by one browser out of hit points it watched change --
	// no event carries a splatter and no row records one -- so this column is
	// read by the page that renders the tabletop and honoured entirely on the
	// client. See the decals in server/js/room/render/decals.ts.
	//
	// ITS ZERO VALUE IS WRONG IN THE SAME WAY FollowTurn's IS, and for the same
	// reason: on is the default, so a Preferences assembled field by field
	// somewhere other than New quietly turns it off.
	ShowBlood bool

	// PingVolume is how loud a ping is on this reader's tabletop, as a
	// percentage of the sound's own level, and zero is silence.
	//
	// IT IS THE THIRD SETTING HERE ABOUT A CANVAS RATHER THAN A PAGE and it is
	// the account's for FollowTurn's reason: a GM does not get to decide how
	// much noise comes out of somebody else's laptop.
	//
	// NOTHING ON THE SERVER OBEYS IT AND THERE IS NOTHING THERE THAT COULD. A
	// ping is one transient event carrying a point and a person; the noise is
	// synthesised by the browser that receives it, so no event and no row knows
	// a sound happened. See server/js/room/ping-sound.ts.
	//
	// ITS ZERO VALUE IS WRONG IN THE SAME WAY THE TWO BOOLEANS ABOVE ARE, and
	// worse: a Preferences assembled field by field somewhere other than New is
	// not merely quieter, it is silent.
	PingVolume int
}

// New normalises the stored columns into Preferences, falling back field by
// field. It never fails, because it is the read path: a value the database
// holds that this build does not recognise -- a column rolled forward and the
// binary rolled back, a zone dropped from the curated list -- should render a
// slightly wrong date rather than refuse to render the page.
//
// The write path is the parsers below, which do report an unknown value, so
// nothing unrecognised gets stored in the first place.
func New(theme, timezone, dateFormat, timeFormat string, followTurn, showBlood bool, pingVolume int) Preferences {
	p := Default

	if v, ok := ParseTheme(theme); ok {
		p.Theme = v
	}
	if v, ok := ParseTimezone(timezone); ok {
		p.Timezone = v
	}
	if v, ok := ParseDateFormat(dateFormat); ok {
		p.DateFormat = v
	}
	if v, ok := ParseTimeFormat(timeFormat); ok {
		p.TimeFormat = v
	}

	// A BOOLEAN HAS NOTHING TO NORMALISE, which is why these are assignments
	// standing beside four parses. They are in here rather than set by the
	// caller afterwards so that every Preferences in the app is built by one
	// function: a field left to the caller is a field the second caller
	// forgets, and the zero value of either of these silently turns the setting
	// off.
	p.FollowTurn = followTurn
	p.ShowBlood = showBlood

	// AND THE VOLUME IS CLAMPED RATHER THAN REJECTED, which is this function's
	// contract: it is the read path, so a column this build does not like should
	// render a slightly wrong page rather than refuse to render one. A byte
	// holding 120 is a slider at full; a negative one cannot reach here through
	// the database and is silence if it ever does.
	p.PingVolume = ClampPingVolume(pingVolume)

	return p
}

// ClampPingVolume brings a number into range. It is exported because the room
// page reads the stored value straight onto the tabletop and wants the same
// answer New would have given.
func ClampPingVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > PingVolumeMax {
		return PingVolumeMax
	}

	return v
}

// Themes, DateFormats and TimeFormats are the members in the order the settings
// dialog offers them, which is not the order the ENUMs declare: the ENUM is
// append-only so its order is a history of when each was added, and this is a
// list a person reads.
func Themes() []Theme { return []Theme{ThemeSystem, ThemeLight, ThemeDark} }

func DateFormats() []DateFormat {
	return []DateFormat{DateDMYText, DateMDYText, DateMDYSlash, DateDMYSlash, DateISO}
}

func TimeFormats() []TimeFormat { return []TimeFormat{Time12H, Time24H} }

// ParseTheme, ParseDateFormat and ParseTimeFormat report whether a string is a
// member, which is what the settings form validates against. Each walks its own
// ordered list rather than carrying a second map, because all three are shorter
// than the map lookup that would replace them.
func ParseTheme(s string) (Theme, bool) {
	for _, t := range Themes() {
		if string(t) == s {
			return t, true
		}
	}
	return Default.Theme, false
}

func ParseDateFormat(s string) (DateFormat, bool) {
	for _, d := range DateFormats() {
		if string(d) == s {
			return d, true
		}
	}
	return Default.DateFormat, false
}

func ParseTimeFormat(s string) (TimeFormat, bool) {
	for _, t := range TimeFormats() {
		if string(t) == s {
			return t, true
		}
	}
	return Default.TimeFormat, false
}

// ParsePingVolume reads the slider's posted value, reporting whether it was one
// of the positions the form offers.
//
// IT IS STRICTER THAN ClampPingVolume ON PURPOSE, and the split is the one the
// three parsers above already draw: this is the WRITE path, where a value that
// is not a position the page rendered means a stale page or a hand-made request
// and is worth saying so about. The read path clamps, because by then the number
// is already stored and refusing to draw the room over it helps nobody.
//
// AN EMPTY STRING IS THE DEFAULT AND NOT A REFUSAL. A dialog that does not carry
// this field posts nothing for it, and reading that as zero would mute a setting
// nobody was asked about -- which is the trap the two checkboxes above document.
func ParsePingVolume(s string) (int, bool) {
	if s == "" {
		return Default.PingVolume, true
	}

	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > PingVolumeMax || v%PingVolumeStep != 0 {
		return Default.PingVolume, false
	}

	return v, true
}

// Layout is the Go reference-time layout for a date on its own. It is exported
// because the settings dialog labels each option with today's date rendered in
// it -- a select listing "DD/MM/YYYY" makes the reader parse a notation, and one
// listing "06/09/2026" shows them the answer.
//
// PADDING IS NOT UNIFORM AND THAT IS DELIBERATE. The numeric formats zero-pad
// so a column of them lines up; the textual ones do not, because "Sep 06, 2026"
// is how a machine writes a date and "Sep 6, 2026" is how a person does.
func (d DateFormat) Layout() string {
	switch d {
	case DateMDYText:
		return "Jan 2, 2006"
	case DateMDYSlash:
		return "01/02/2006"
	case DateDMYSlash:
		return "02/01/2006"
	case DateISO:
		return "2006-01-02"
	default:
		return "2 Jan 2006"
	}
}

// Format renders at in this date format alone, with no time and no zone. The
// caller converts to the zone it wants first.
func (d DateFormat) Format(at time.Time) string {
	return at.Format(d.Layout())
}

// Layout is the Go reference-time layout for a time of day.
func (t TimeFormat) Layout() string {
	if t == Time24H {
		return "15:04"
	}
	return "3:04 PM"
}

// Location is the zone to render in, and it is never nil. An unknown name
// resolves to the default zone rather than to UTC, so a stored value this build
// does not have still lands in a plausible place instead of silently shifting
// every date by five hours.
func (p Preferences) Location() *time.Location {
	if loc, ok := zone(p.Timezone); ok {
		return loc
	}
	if loc, ok := zone(DefaultTimezone); ok {
		return loc
	}
	return time.UTC
}

// Format renders one instant twice: the RFC 3339 value for a machine, and the
// reader's own rendering for a person.
//
// ISO IS ALWAYS UTC AND NEVER FOLLOWS THE PREFERENCES. It is the machine-
// readable instant that lands in a datetime attribute, and an instant is the
// same everywhere; moving it into the reader's zone would encode a preference
// into a value that exists precisely so no preference is needed to read it.
//
// THE ZONE ABBREVIATION IS NOT OPTIONAL. Until now every rendered date was
// stamped UTC and the browser rewrote it into the reader's own zone, so "whose
// clock is this?" had one answer. Now the server picks the zone, and a bare
// "2:04 PM" on a page a reader opens away from home is a time with no owner.
//
// A zone with no abbreviation renders a numeric offset instead -- Kathmandu
// comes out "+0545" rather than three letters -- which is correct and is why
// nothing here assumes a length.
func (p Preferences) Format(at time.Time) (iso string, text string) {
	layout := p.DateFormat.Layout() + ", " + p.TimeFormat.Layout() + " MST"

	return at.UTC().Format(time.RFC3339), at.In(p.Location()).Format(layout)
}
