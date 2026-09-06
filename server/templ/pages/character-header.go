package pages

import "strings"

// CharacterHeader is what the bar across the top of every editor tab renders.
//
// IT IS ON ALL FIVE TAB PAGES BECAUSE THE BAR IS, and the bar is because the
// editor used to head every tab with the words "Edit Character" -- a title that
// says which screen you are on and nothing about whose sheet is open. The
// character's name existed on that page exactly once, as the value of the first
// text input.
//
// IT DOES NOT CARRY THE CHARACTER'S ID. Every page data struct already has one,
// and the tab strip inside the bar is built from that -- so a second copy here
// would be a field that could disagree with the one the panels post to, and a
// header built without it would render four tabs linking to /characters//edit.
// The shell takes the id and the bar beside each other for exactly that reason.
//
// Every field is a rendered string rather than the column behind it. The bar
// prints numbers and never reads them back, so formatting here keeps the markup
// free of conversions and lets the empty struct render an empty bar -- which is
// what the concurrency test does to every page.
type CharacterHeader struct {
	Name string
	// Subtitle is species, class, background and alignment joined with
	// separators, built by the controller because it is the only place that
	// knows which of the four the row actually has.
	Subtitle string
	// AvatarID is empty when the character has no portrait, and the bar falls
	// back to the initial the roster card already uses.
	AvatarID string

	// The six chips. AC, hit points and speed are stored columns; initiative,
	// proficiency and passive perception are worked out. The bar does not say
	// which is which, because a player reading their own AC does not care.
	AC          string
	CurrentHP   string
	MaxHP       string
	Speed       string
	Initiative  string
	Proficiency string
	Passive     string
}

// characterBarName covers the character whose name is blank. The Identity panel
// requires one, so this is reachable only between a row being created and the
// first save -- but creation takes a name too, and an empty bar heading reads as
// a broken page rather than as an unnamed character.
func characterBarName(header CharacterHeader) string {
	if strings.TrimSpace(header.Name) == "" {
		return "Unnamed character"
	}

	return header.Name
}

// AlignmentLabel turns a stored alignment into the words the picker shows.
// Exported because the subtitle is assembled in the controller, next to the rest
// of the row it is read from.
//
// An unrecognised value comes back empty rather than as itself: the column is
// free text as far as MySQL is concerned, and the bar is not the place a value
// nothing wrote gets its first airing.
func AlignmentLabel(value string) string {
	for _, option := range alignmentOptions {
		if option.Value == value {
			return option.Label
		}
	}

	return ""
}

// headerChip is one reading on the bar. Sub is the smaller half of a value that
// has two parts -- 31 / 38 hit points, 30 ft. of speed -- and is empty for the
// four that are a single number.
//
// IT CARRIES ITS OWN SEPARATOR. The markup renders Value and Sub as siblings
// with the one space between them that any two inline elements get, so the
// slash belongs to the hit points rather than to the space between them --
// which is what makes "31 / 38" and "30 ft." come out of the same two fields.
type headerChip struct {
	Label string
	Value string
	Sub   string
}

// characterChips is the order the bar reads in, and it is Go rather than markup
// because the list is data: six entries, one of which is assembled from two
// fields and one of which is split back apart.
func characterChips(header CharacterHeader) []headerChip {
	speed, speedUnit := splitMeasurement(header.Speed)

	return []headerChip{
		{Label: "AC", Value: header.AC},
		{Label: "Hit Points", Value: header.CurrentHP, Sub: "/ " + header.MaxHP},
		{Label: "Speed", Value: speed, Sub: speedUnit},
		{Label: "Initiative", Value: header.Initiative},
		{Label: "Proficiency", Value: header.Proficiency},
		{Label: "Passive", Value: header.Passive},
	}
}

// splitMeasurement pulls the leading number off a stored speed so the chip can
// set "30" in the size every other reading uses and "ft." in the size a unit
// deserves.
//
// The column is free text and always has been -- a character with a fly speed
// writes two of them into it -- so anything that does not start with a digit
// comes back whole and unstyled rather than being made to fit.
func splitMeasurement(value string) (string, string) {
	value = strings.TrimSpace(value)

	digits := 0
	for digits < len(value) && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return value, ""
	}

	return value[:digits], strings.TrimSpace(value[digits:])
}
