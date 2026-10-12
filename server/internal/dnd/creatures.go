package dnd

var alignments = []Option{
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
var sizes = []Option{
	{Label: "Tiny", Value: "tiny"},
	{Label: "Small", Value: "small"},
	{Label: "Medium", Value: "medium"},
	{Label: "Large", Value: "large"},
	{Label: "Huge", Value: "huge"},
	{Label: "Gargantuan", Value: "gargantuan"},
}
var creatureTypes = []Option{
	{Label: "Aberration", Value: "aberration"},
	{Label: "Beast", Value: "beast"},
	{Label: "Celestial", Value: "celestial"},
	{Label: "Construct", Value: "construct"},
	{Label: "Dragon", Value: "dragon"},
	{Label: "Elemental", Value: "elemental"},
	{Label: "Fey", Value: "fey"},
	{Label: "Fiend", Value: "fiend"},
	{Label: "Giant", Value: "giant"},
	{Label: "Humanoid", Value: "humanoid"},
	{Label: "Monstrosity", Value: "monstrosity"},
	{Label: "Ooze", Value: "ooze"},
	{Label: "Plant", Value: "plant"},
	{Label: "Undead", Value: "undead"},
}

const (
	DefaultAlignment    = "unaligned"
	DefaultSize         = "medium"
	DefaultCreatureType = "humanoid"
)

func Alignments() []Option    { return alignments }
func Sizes() []Option         { return sizes }
func CreatureTypes() []Option { return creatureTypes }
func SizeValues() []string {
	values := make([]string, 0, len(sizes))
	for _, size := range sizes {
		values = append(values, size.Value)
	}
	return values
}
func NormalizeAlignment(value string) string {
	return normalize(value, alignments, DefaultAlignment)
}
func NormalizeSize(value string) string {
	return normalize(value, sizes, DefaultSize)
}
func NormalizeCreatureType(value string) string {
	return normalize(value, creatureTypes, DefaultCreatureType)
}
func AlignmentLabel(value string) string    { return label(value, alignments) }
func SizeLabel(value string) string         { return label(value, sizes) }
func CreatureTypeLabel(value string) string { return label(value, creatureTypes) }

type ChallengeRating struct {
	Value       string
	Label       string
	XP          uint32
	Proficiency uint8
}

const (
	DefaultChallengeRating = "0"
	CRZeroArmedXP          = 10
)

var challengeRatings = []ChallengeRating{
	{Value: "0", Label: "0", XP: 0, Proficiency: 2},
	{Value: "1/8", Label: "1/8", XP: 25, Proficiency: 2},
	{Value: "1/4", Label: "1/4", XP: 50, Proficiency: 2},
	{Value: "1/2", Label: "1/2", XP: 100, Proficiency: 2},
	{Value: "1", Label: "1", XP: 200, Proficiency: 2},
	{Value: "2", Label: "2", XP: 450, Proficiency: 2},
	{Value: "3", Label: "3", XP: 700, Proficiency: 2},
	{Value: "4", Label: "4", XP: 1100, Proficiency: 2},
	{Value: "5", Label: "5", XP: 1800, Proficiency: 3},
	{Value: "6", Label: "6", XP: 2300, Proficiency: 3},
	{Value: "7", Label: "7", XP: 2900, Proficiency: 3},
	{Value: "8", Label: "8", XP: 3900, Proficiency: 3},
	{Value: "9", Label: "9", XP: 5000, Proficiency: 4},
	{Value: "10", Label: "10", XP: 5900, Proficiency: 4},
	{Value: "11", Label: "11", XP: 7200, Proficiency: 4},
	{Value: "12", Label: "12", XP: 8400, Proficiency: 4},
	{Value: "13", Label: "13", XP: 10000, Proficiency: 5},
	{Value: "14", Label: "14", XP: 11500, Proficiency: 5},
	{Value: "15", Label: "15", XP: 13000, Proficiency: 5},
	{Value: "16", Label: "16", XP: 15000, Proficiency: 5},
	{Value: "17", Label: "17", XP: 18000, Proficiency: 6},
	{Value: "18", Label: "18", XP: 20000, Proficiency: 6},
	{Value: "19", Label: "19", XP: 22000, Proficiency: 6},
	{Value: "20", Label: "20", XP: 25000, Proficiency: 6},
	{Value: "21", Label: "21", XP: 33000, Proficiency: 7},
	{Value: "22", Label: "22", XP: 41000, Proficiency: 7},
	{Value: "23", Label: "23", XP: 50000, Proficiency: 7},
	{Value: "24", Label: "24", XP: 62000, Proficiency: 7},
	{Value: "25", Label: "25", XP: 75000, Proficiency: 8},
	{Value: "26", Label: "26", XP: 90000, Proficiency: 8},
	{Value: "27", Label: "27", XP: 105000, Proficiency: 8},
	{Value: "28", Label: "28", XP: 120000, Proficiency: 8},
	{Value: "29", Label: "29", XP: 135000, Proficiency: 9},
	{Value: "30", Label: "30", XP: 155000, Proficiency: 9},
}

func ChallengeRatings() []ChallengeRating { return challengeRatings }
func ChallengeRatingOptions() []Option {
	options := make([]Option, 0, len(challengeRatings))
	for _, rating := range challengeRatings {
		options = append(options, Option{Label: rating.Label, Value: rating.Value})
	}
	return options
}
func ChallengeRatingLabel(value string) string {
	for _, rating := range challengeRatings {
		if rating.Value == value {
			return rating.Label
		}
	}
	return ""
}
func NormalizeChallengeRating(value string) string {
	for _, rating := range challengeRatings {
		if rating.Value == value {
			return value
		}
	}
	return DefaultChallengeRating
}
func ChallengeRatingNumbers(value string) (xp uint32, proficiency uint8, nextXP uint32) {
	index := 0
	for i, rating := range challengeRatings {
		if rating.Value == value {
			index = i
			break
		}
	}
	if index+1 < len(challengeRatings) {
		nextXP = challengeRatings[index+1].XP
	}
	return challengeRatings[index].XP, challengeRatings[index].Proficiency, nextXP
}
