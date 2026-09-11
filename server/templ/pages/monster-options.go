package pages
var creatureTypeOptions = []Option{
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
const DefaultCreatureType = "humanoid"
func NormalizeCreatureType(value string) string {
	for _, option := range creatureTypeOptions {
		if option.Value == value {
			return value
		}
	}
	return DefaultCreatureType
}
func CreatureTypeLabel(value string) string {
	for _, option := range creatureTypeOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return ""
}
type ChallengeRating struct {
	Value       string
	Label       string
	XP          uint32
	Proficiency uint8
}
const DefaultChallengeRating = "0"
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
func challengeRatingOptions() []Option {
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
const (
	MonsterActionKindTrait           = "trait"
	MonsterActionKindAction          = "action"
	MonsterActionKindBonusAction     = "bonus_action"
	MonsterActionKindReaction        = "reaction"
	MonsterActionKindLegendaryAction = "legendary_action"
	MonsterActionKindLairAction      = "lair_action"
	MonsterActionKindRegionalEffect  = "regional_effect"
)
type MonsterActionSection struct {
	Kind     string
	Heading  string
	Singular string
	Intro    string
}
func (s MonsterActionSection) AddLabel() string {
	return "Add " + s.Singular
}
var monsterActionKinds = []MonsterActionSection{
	{
		Kind:     MonsterActionKindTrait,
		Heading:  "Traits",
		Singular: "Trait",
	},
	{
		Kind:     MonsterActionKindAction,
		Heading:  "Actions",
		Singular: "Action",
	},
	{
		Kind:     MonsterActionKindBonusAction,
		Heading:  "Bonus Actions",
		Singular: "Bonus Action",
	},
	{
		Kind:     MonsterActionKindReaction,
		Heading:  "Reactions",
		Singular: "Reaction",
	},
	{
		Kind:     MonsterActionKindLegendaryAction,
		Heading:  "Legendary Actions",
		Singular: "Legendary Action",
		Intro:    "Immediately after another creature's turn, it can expend a use to take one of the following actions. It regains all expended uses at the start of each of its turns.",
	},
	{
		Kind:     MonsterActionKindLairAction,
		Heading:  "Lair Actions",
		Singular: "Lair Action",
		Intro:    "On initiative count 20 (losing initiative ties), it takes a lair action to cause one of the following effects. It can't use the same effect two rounds in a row.",
	},
	{
		Kind:     MonsterActionKindRegionalEffect,
		Heading:  "Regional Effects",
		Singular: "Regional Effect",
		Intro:    "The region containing its lair is warped by its magic, creating one or more of the following effects.",
	},
}
func MonsterActionSections() []MonsterActionSection { return monsterActionKinds }
func MonsterActionSectionFor(kind string) (MonsterActionSection, bool) {
	for _, section := range monsterActionKinds {
		if section.Kind == kind {
			return section, true
		}
	}
	return MonsterActionSection{}, false
}
const (
	MonsterNameLimit      = 128
	MonsterTagsLimit      = 128
	MonsterHitDiceLimit   = 64
	MonsterSpeedLimit     = 128
	MonsterDefenseLimit   = 512
	MonsterSensesLimit    = 255
	MonsterLanguagesLimit = 255
	MonsterHabitatLimit   = 255
	MonsterTreasureLimit  = 64
	MonsterActionNameLimit = 128
)
const MonsterProseLimit = 4096
const (
	MonsterACLimit            = 255
	MonsterHPLimit            = 9999
	MonsterLegendaryUsesLimit = 255
)
