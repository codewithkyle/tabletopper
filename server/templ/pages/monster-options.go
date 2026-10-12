package pages

import "tabletopper/internal/dnd"

var creatureTypeOptions = dnd.CreatureTypes()

func challengeRatingOptions() []Option { return dnd.ChallengeRatingOptions() }

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
	MonsterNameLimit       = 128
	MonsterTagsLimit       = 128
	MonsterHitDiceLimit    = 64
	MonsterSpeedLimit      = 128
	MonsterDefenseLimit    = 512
	MonsterSensesLimit     = 255
	MonsterLanguagesLimit  = 255
	MonsterHabitatLimit    = 255
	MonsterTreasureLimit   = 64
	MonsterActionNameLimit = 128
)
const MonsterProseLimit = 4096
const (
	MonsterACLimit            = 255
	MonsterHPLimit            = 9999
	MonsterLegendaryUsesLimit = 255
)
