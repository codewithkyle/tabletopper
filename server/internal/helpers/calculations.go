package helpers

import (
	"math"
	"strconv"
)

var dnd5eXPThresholds = [...]uint32{
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

func CalculateModifier(base int) string {
	modifier := math.Floor(float64(base-10) / 2)
	modifierStr := strconv.Itoa(int(modifier))
	if modifier >= 0 {
		return "+" + modifierStr
	} else {
		return modifierStr
	}
}

func CalculateProficiencyBonus(cr int) string {
	if cr >= 0 && cr <= 4 {
		return "+2"
	} else if cr >= 5 && cr <= 8 {
		return "+3"
	} else if cr >= 9 && cr <= 12 {
		return "+4"
	} else if cr >= 13 && cr <= 16 {
		return "+5"
	} else if cr >= 17 && cr <= 20 {
		return "+6"
	} else if cr >= 21 && cr <= 24 {
		return "+7"
	} else if cr >= 26 && cr <= 28 {
		return "+8"
	} else {
		return "+9"
	}
}

func CalculateCharacterLevelFromXP(xp uint32) uint8 {
	for level := len(dnd5eXPThresholds) - 1; level >= 0; level-- {
		if xp >= dnd5eXPThresholds[level] {
			return uint8(level + 1)
		}
	}

	return 1
}

func CalculateCharacterProficiencyBonus(level uint8) uint16 {
	if level <= 4 {
		return 2
	} else if level <= 8 {
		return 3
	} else if level <= 12 {
		return 4
	} else if level <= 16 {
		return 5
	}

	return 6
}
