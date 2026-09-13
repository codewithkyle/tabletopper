package pages

import (
	"strconv"

	"tabletopper/internal/prefs"
)

const accountSettingsID = "account-settings"
const AccountSettingsPanel = "account-settings"
const AccountSettingsPath = "/fragment/account/settings"
const (
	accountWelcomeID    = "account-welcome"
	AccountWelcomePanel = "account-welcome"
)
const DisplayNameLimit = 128

type ZoneGroup struct {
	Label string
	Zones []ZoneOption
}
type ZoneOption struct {
	Value string
	Label string
	Alias string
}
type AccountSettingsData struct {
	Name        string
	Themes      []Option
	Theme       string
	Zones       []ZoneGroup
	Zone        string
	DateFormats []Option
	DateFormat  string
	TimeFormats []Option
	TimeFormat  string
	FollowTurn  bool
	ShowBlood   bool
	PingVolume  int
	TurnAlert   bool
	TurnVolume  int
	TurnNotify  bool
	Storage     string
}

var (
	VolumeMax  = strconv.Itoa(prefs.VolumeMax)
	VolumeStep = strconv.Itoa(prefs.VolumeStep)
)

const (
	PingVolumeOutputID = "ping-volume-value"
	TurnVolumeOutputID = "turn-volume-value"
)

func (d AccountSettingsData) PingVolumeValue() string {
	return strconv.Itoa(d.PingVolume)
}
func (d AccountSettingsData) TurnVolumeValue() string {
	return strconv.Itoa(d.TurnVolume)
}
