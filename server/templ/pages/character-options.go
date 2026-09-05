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

// The values the new-character form starts on. Both match the fallback
// characterToEditPageData uses when the stored column is empty, so a character
// saved straight from the blank form round-trips to the same selection.
const (
	defaultAlignment = "unaligned"
	defaultSize      = "medium"
)
