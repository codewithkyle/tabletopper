package pages

import (
	"tabletopper/internal/dnd"
	"tabletopper/internal/room"
	"tabletopper/internal/share"
)

const (
	ACLimit           = room.ACLimit
	HPLimit           = room.HPLimit
	ObjectPixelsMax   = room.ObjectPixelsMax
	CharacterXPLimit  = dnd.CharacterXPLimit
	AbilityScoreLimit = dnd.AbilityScoreLimit
	ShareMinDays      = 1
	ShareMaxDays      = 365
	SharePasswordMin  = share.PasswordMin
	SharePasswordMax  = share.PasswordMax
	SpellSlotLimit    = 99
)
