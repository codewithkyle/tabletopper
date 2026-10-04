package pages

import (
	"tabletopper/internal/room"
	"tabletopper/internal/share"
)

const (
	ACLimit           = room.ACLimit
	HPLimit           = room.HPLimit
	ObjectPixelsMax   = room.ObjectPixelsMax
	CharacterXPLimit  = 9_999_999
	AbilityScoreLimit = 255
	ShareMinDays      = 1
	ShareMaxDays      = 365
	SharePasswordMin  = share.PasswordMin
	SharePasswordMax  = share.PasswordMax
)
