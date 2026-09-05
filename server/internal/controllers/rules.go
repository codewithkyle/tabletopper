package controllers

// The two pieces of 5e arithmetic the character form derives rather than
// asks for. Level follows from XP and proficiency follows from level, so
// neither is a field the user can set.

// xpThresholds[i] is the XP at which a character reaches level i+1.
var xpThresholds = [...]uint32{
	0,
	300,
	900,
	2700,
	6500,
	14000,
	23000,
	34000,
	48000,
	64000,
	85000,
	100000,
	120000,
	140000,
	165000,
	195000,
	225000,
	265000,
	305000,
	355000,
}

func levelFromXP(xp uint32) uint8 {
	for level := len(xpThresholds) - 1; level >= 0; level-- {
		if xp >= xpThresholds[level] {
			return uint8(level + 1)
		}
	}

	return 1
}

func proficiencyBonusForLevel(level uint8) uint16 {
	switch {
	case level <= 4:
		return 2
	case level <= 8:
		return 3
	case level <= 12:
		return 4
	case level <= 16:
		return 5
	}

	return 6
}
