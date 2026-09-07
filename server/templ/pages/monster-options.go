package pages

// The closed sets a monster picks from, and the numbers that follow from one of
// them. Every list here is read twice -- once by the select that offers it and
// once by the normaliser that checks what came back -- which is what keeps a
// posted value that is not on the list from reaching a column, and what stops
// the two copies drifting apart.
//
// The monster editor reuses the character sheet's own lists wherever the answer
// is the same one: sizeOptions, alignmentOptions and proficiencyOptions are
// shared as they stand. What is here is the three sets a stat block has and a
// character sheet does not.

// The fourteen creature types, which is the whole of the list in both editions.
// Lower-case values like every other stored choice in this package, so the
// column holds `humanoid` and the subtitle prints "Humanoid".
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

// DefaultCreatureType is what the schema writes and what an unrecognised value
// falls back to. Humanoid because most of what a GM writes down is a person:
// bandits, cultists, guards and the nobles who hire them.
const DefaultCreatureType = "humanoid"

// NormalizeCreatureType is the allowlist behind the select, the shape
// NormalizeProficiency has. The column is a VARCHAR rather than an ENUM for the
// reason spells.school is: this runs anyway to check what a select posted, and
// an ENUM would be a second copy of the list that answers a bad value with a
// 500 instead of a correction.
func NormalizeCreatureType(value string) string {
	for _, option := range creatureTypeOptions {
		if option.Value == value {
			return value
		}
	}

	return DefaultCreatureType
}

// CreatureTypeLabel turns a stored type into the word the subtitle prints, the
// way SizeLabel and AlignmentLabel do for the other two thirds of that line. An
// unrecognised value comes back empty rather than as itself: the column is free
// text as far as MySQL is concerned, and a stat block is not the place a value
// nothing wrote gets its first airing.
func CreatureTypeLabel(value string) string {
	for _, option := range creatureTypeOptions {
		if option.Value == value {
			return option.Label
		}
	}

	return ""
}

// ChallengeRating is one rating and everything that follows from it.
//
// THE XP AND THE PROFICIENCY BONUS ARE NOT COLUMNS. They are functions of the
// rating, so a stat block works them out at render time -- the old monsters
// table stored xp and a GM who changed a monster's CR had two numbers to fix,
// one of which they would forget. This slice is the whole of that arithmetic:
// the select, the validator and the stat block read the same thirty-four lines
// and cannot disagree.
//
// Label is what a picker and a stat block print; Value is what the column
// holds. They are the same characters for every entry today, because a rating
// is stored the way it is printed -- but every other closed set in this package
// prints a label rather than a stored value, and a set that read its own values
// into markup would be the one place a stored string reaches a reader unedited.
type ChallengeRating struct {
	Value       string
	Label       string
	XP          uint32
	Proficiency uint8
}

// DefaultChallengeRating is what the schema writes and what an unrecognised
// value falls back to.
const DefaultChallengeRating = "0"

// The thirty-four ratings, in order, which is the order the select offers and
// the order the in-lair figure steps through: a monster with lair actions is
// one rating harder inside its lair, so that figure is the XP of the NEXT entry
// in this slice. CR 30 has no next entry and prints none.
//
// The proficiency bonus is +2 up to CR 4 and gains one every four ratings after
// it, to +9 at CR 29. The three fractions are all +2 and all below CR 1, so
// they sit inside the first band with nothing special about them.
//
// CR 0 IS THE ONE RATING WITH TWO XP VALUES in the rules -- 0 for a creature
// with no damaging attack and 10 for one that has -- and neither is stored. The
// 0 is here and the stat block answers 10 when the monster has at least one row
// in its Actions section, which is the closest thing the editor has to the
// question the rules are actually asking.
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

// ChallengeRatings is read by the controller, which needs the XP and the
// proficiency bonus, and by the select, which needs the labels. One list, in
// one package, the way SkillEntries is.
func ChallengeRatings() []ChallengeRating { return challengeRatings }

// challengeRatingOptions is the same thirty-four entries as the picker reads
// them. It is built from the list rather than written beside it, so a rating
// cannot exist in the arithmetic and be missing from the select -- which is the
// failure that would look like a monster whose CR reverts every time it saves.
func challengeRatingOptions() []Option {
	options := make([]Option, 0, len(challengeRatings))
	for _, rating := range challengeRatings {
		options = append(options, Option{Label: rating.Label, Value: rating.Value})
	}

	return options
}

// ChallengeRatingLabel turns a stored rating into what the block prints after
// "CR" and what the bar's chip shows, the way SizeLabel and CreatureTypeLabel do
// for the subtitle. An unrecognised value comes back empty rather than as
// itself, which is the rule every label in this package follows.
func ChallengeRatingLabel(value string) string {
	for _, rating := range challengeRatings {
		if rating.Value == value {
			return rating.Label
		}
	}

	return ""
}

// NormalizeChallengeRating is the allowlist behind the CR select.
func NormalizeChallengeRating(value string) string {
	for _, rating := range challengeRatings {
		if rating.Value == value {
			return value
		}
	}

	return DefaultChallengeRating
}

// The seven sections of a stat block, as the words that ride in a URL. They are
// the members of the monster_actions.kind ENUM, and they have to stay those
// exact strings: a row's kind is half the route it posts to and half the index
// it is read back by.
const (
	MonsterActionKindTrait           = "trait"
	MonsterActionKindAction          = "action"
	MonsterActionKindBonusAction     = "bonus_action"
	MonsterActionKindReaction        = "reaction"
	MonsterActionKindLegendaryAction = "legendary_action"
	MonsterActionKindLairAction      = "lair_action"
	MonsterActionKindRegionalEffect  = "regional_effect"
)

// MonsterActionSection is one of those seven: the word in the URL, the heading
// it renders under, what one of its rows is called, and the sentence the book
// opens it with where it has one.
//
// THE INTRO IS PART OF THE SECTION AND NOT PART OF ANY ROW. Three of the seven
// open with a standing rule -- how legendary actions are spent, when lair
// actions happen, what a region around a lair is like -- and the book prints
// each of them once, above the list, rather than on every entry.
//
// Singular is the heading in the singular, and it is stored rather than derived
// from it because English does not take the "s" off Regional Effects the way a
// rule would. Two things read it: the add button and the toast a nameless row
// saves with, and neither should be a second place the word is written.
type MonsterActionSection struct {
	Kind     string
	Heading  string
	Singular string
	Intro    string
}

// AddLabel is what the button under a section says.
func (s MonsterActionSection) AddLabel() string {
	return "Add " + s.Singular
}

// monsterActionKinds is the allowlist and the section order in one list. It is
// ranged over by the editor, so a section cannot be left off the page by
// forgetting to write its markup, and it is looked up by the handlers, so a
// kind that is not one of these never reaches a statement.
//
// THE ORDER IS THE ENUM'S ORDER, which is the stat block's order, and the two
// have to match: rows come back ORDER BY kind, and MySQL sorts an ENUM by the
// order its members were declared. TestEveryActionKindHasASection reads the
// schema dump and holds them together.
//
// THE THREE SENTENCES SAY "IT" WHERE THE BOOK NAMES THE CREATURE. The book
// writes "the dragon can expend a use" under a heading that already says Ancient
// Red Dragon; a stat block here has no short noun to put there, and building one
// out of the name would produce "the Bandit Captain Nightshade can expend a
// use". The pronoun is the smaller loss.
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

// MonsterActionSections is the list the editor ranges over to draw its seven
// panels and the stat block ranges over to draw its sections.
func MonsterActionSections() []MonsterActionSection { return monsterActionKinds }

// MonsterActionSectionFor is the allowlist gate. A kind that is not one of the
// seven comes back not-ok, and the handler answers 404 without running a
// statement -- the shape bonusPanels has for the one other route that takes a
// segment of its path as a name.
func MonsterActionSectionFor(kind string) (MonsterActionSection, bool) {
	for _, section := range monsterActionKinds {
		if section.Kind == kind {
			return section, true
		}
	}

	return MonsterActionSection{}, false
}

// The column widths, which the handlers enforce and the maxlength attributes
// render. They are here rather than in the controller for the reason the vitals
// bounds are: a max in the markup and a range check in the handler are the same
// fact written twice, and the copy in the template is the one that drifts
// silently. controllers can read this package and this package cannot read
// controllers, so this is the end the shared number lives at.
//
// THE VARCHARS ARE COUNTED IN CHARACTERS AND THE TEXT COLUMNS IN BYTES, which
// is what MySQL counts in each. Measuring a name with len() would refuse ninety
// accented letters the column would have taken.
//
// MonsterDefenseLimit covers four columns rather than one because they are one
// answer repeated: vulnerabilities, resistances, immunities and gear are all
// 512-character lines of comma-separated words, and four constants of the same
// number would be four things to keep in step.
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

// MonsterProseLimit is the one cap measured in bytes, because the two columns it
// guards are TEXT and TEXT counts bytes. It is 4 KB rather than the column's
// 64 KB for the reason characterProseLimit is: 4 KB is already several pages of
// notes, and the boxes carry maxlength="1024", so 1024 UTF-16 units cannot
// encode past 3 KB and the cap is reachable only by a request nobody's browser
// made.
const MonsterProseLimit = 4096

// The numeric bounds, which the markup renders into max attributes and the
// handlers refuse past. Each is its column's own ceiling rather than a rule
// somebody made up: ac is TINYINT UNSIGNED and so is the legendary count, and
// the rules put no number on either.
//
// Hit points are the exception and are capped below the column's 65535, at the
// same 9999 the character sheet's hit point boxes carry. Nothing in print comes
// near it -- the Tarrasque has 697 -- and a four-digit box is what makes the
// number readable at a glance.
const (
	MonsterACLimit            = 255
	MonsterHPLimit            = 9999
	MonsterLegendaryUsesLimit = 255
)
