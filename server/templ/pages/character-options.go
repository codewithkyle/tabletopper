package pages

// Option is one entry in a selectField dropdown.
type Option struct {
	Label string
	Value string
}

// The two fixed dropdown lists on the character sheet. They used to be JSON
// blobs duplicated verbatim in the data-options attribute of both
// new-character.templ and edit-character.templ and parsed client-side by
// select.js; now they are one Go slice each, ranged over at render time.
//
// The "Unaligned" value was stored as "unaliged" until the migration of
// 2026-09-05 corrected every row; characterToEditPageData falls back to the
// same spelling, so the two have to move together.
var alignmentOptions = []Option{
	{Label: "Unaligned", Value: "unaligned"},
	{Label: "Any Alignment", Value: "any alignment"},
	{Label: "Lawful Good", Value: "lawful good"},
	{Label: "Neutral Good", Value: "neutral good"},
	{Label: "Chaotic Good", Value: "chaotic good"},
	{Label: "Lawful Neutral", Value: "lawful neutral"},
	{Label: "Neutral", Value: "neutral"},
	{Label: "Chaotic Neutral", Value: "chaotic neutral"},
	{Label: "Lawful Evil", Value: "lawful evil"},
	{Label: "Neutral Evil", Value: "neutral evil"},
	{Label: "Chaotic Evil", Value: "chaotic evil"},
	{Label: "Any Neutral Alignment", Value: "any neutral alignment"},
	{Label: "Any Good Alignment", Value: "any good alignment"},
	{Label: "Any Chaotic Alignment", Value: "any chaotic alignment"},
	{Label: "Any Lawful Alignment", Value: "any lawful alignment"},
	{Label: "Any Non-Good Alignment", Value: "any non-good alignment"},
}

var sizeOptions = []Option{
	{Label: "Tiny", Value: "tiny"},
	{Label: "Small", Value: "small"},
	{Label: "Medium", Value: "medium"},
	{Label: "Large", Value: "large"},
	{Label: "Huge", Value: "huge"},
	{Label: "Gargantuan", Value: "gargantuan"},
}

// DefaultAlignment and DefaultSize are what the editor selects when the stored
// column has nothing in it. Alignment reaches that state on every new character,
// because creation writes a name and leaves the column NULL. Size does not --
// the schema defaults it -- but the fallback is kept for a row written before
// that default, or around it.
//
// Exported so characterToEditPageData can apply them, the way DefaultSpellSchool
// already is. That is what keeps each one next to the list it has to be a member
// of: a fallback outside the options renders a picker with nothing selected, and
// "unaligned" was stored as "unaliged" until the migration of 2026-09-05
// corrected every row.
const (
	DefaultAlignment = "unaligned"
	DefaultSize      = "medium"
)

// SizeLabel turns a stored size into the word the picker shows, the way
// AlignmentLabel does for alignment. An unrecognised value comes back empty
// rather than as itself: the column is free text as far as MySQL is concerned,
// and a shared sheet is not the place a value nothing wrote gets its first
// airing.
func SizeLabel(value string) string {
	for _, option := range sizeOptions {
		if option.Value == value {
			return option.Label
		}
	}

	return ""
}
