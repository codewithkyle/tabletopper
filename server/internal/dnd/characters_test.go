package dnd

import "testing"

func TestAbilityModifierMatchesTheRules(t *testing.T) {
	for _, c := range []struct {
		score uint8
		want  int
	}{
		{1, -5}, {2, -4}, {7, -2}, {8, -1}, {9, -1}, {10, 0},
		{11, 0}, {12, 1}, {14, 2}, {15, 2}, {20, 5}, {30, 10},
	} {
		if got := AbilityModifier(c.score); got != c.want {
			t.Errorf("AbilityModifier(%d) = %d, want %d", c.score, got, c.want)
		}
	}
}
func TestProficiencyGrantRoundsHalfDown(t *testing.T) {
	for _, c := range []struct {
		state string
		bonus int
		want  int
	}{
		{ProficiencyNone, 3, 0},
		{ProficiencyHalf, 2, 1},
		{ProficiencyHalf, 3, 1},
		{ProficiencyHalf, 5, 2},
		{ProficiencyProficient, 3, 3},
		{ProficiencyExpertise, 3, 6},
		{"nonsense", 3, 0},
	} {
		if got := ProficiencyGrant(c.state, c.bonus); got != c.want {
			t.Errorf("ProficiencyGrant(%q, %d) = %d, want %d", c.state, c.bonus, got, c.want)
		}
	}
}
func TestLevelAndProficiencyFollowTheXPTable(t *testing.T) {
	for _, c := range []struct {
		xp          uint32
		level       uint8
		proficiency uint16
	}{
		{0, 1, 2}, {299, 1, 2}, {300, 2, 2}, {6499, 4, 2}, {6500, 5, 3},
		{48000, 9, 4}, {100000, 12, 4}, {120000, 13, 5}, {225000, 17, 6},
		{354999, 19, 6}, {355000, 20, 6}, {CharacterXPLimit, 20, 6},
	} {
		level := LevelFromXP(c.xp)
		if level != c.level {
			t.Errorf("%d XP is level %d, want %d", c.xp, level, c.level)
		}
		if got := ProficiencyBonus(level); got != c.proficiency {
			t.Errorf("level %d has a proficiency bonus of +%d, want +%d", level, got, c.proficiency)
		}
	}
}
func TestEveryTableHoldsItsDefaultAndFallsBackToIt(t *testing.T) {
	for name, c := range map[string]struct {
		options   []Option
		fallback  string
		normalize func(string) string
	}{
		"sizes":                  {Sizes(), DefaultSize, NormalizeSize},
		"alignments":             {Alignments(), DefaultAlignment, NormalizeAlignment},
		"creature types":         {CreatureTypes(), DefaultCreatureType, NormalizeCreatureType},
		"challenge ratings":      {ChallengeRatingOptions(), DefaultChallengeRating, NormalizeChallengeRating},
		"proficiencies":          {Proficiencies(), ProficiencyNone, NormalizeProficiency},
		"spellcasting abilities": {SpellcastingAbilities(), SpellcastingAbilityNone, NormalizeSpellcastingAbility},
	} {
		if got := c.normalize("nonsense"); got != c.fallback {
			t.Errorf("%s: an unknown value became %q, want %q", name, got, c.fallback)
		}
		seen := map[string]bool{}
		for _, option := range c.options {
			if seen[option.Value] {
				t.Errorf("%s: %q is listed twice", name, option.Value)
			}
			seen[option.Value] = true
			if got := c.normalize(option.Value); got != option.Value {
				t.Errorf("%s: %q normalized to %q", name, option.Value, got)
			}
		}
		if !seen[c.fallback] {
			t.Errorf("%s: the default %q is not one of the options", name, c.fallback)
		}
	}
}
