package prefs
import (
	"strconv"
	"time"
	_ "time/tzdata"
)
type Theme string
const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)
type DateFormat string
const (
	DateDMYText  DateFormat = "dmy_text"
	DateMDYText  DateFormat = "mdy_text"
	DateMDYSlash DateFormat = "mdy_slash"
	DateDMYSlash DateFormat = "dmy_slash"
	DateISO      DateFormat = "iso"
)
type TimeFormat string
const (
	Time12H TimeFormat = "12h"
	Time24H TimeFormat = "24h"
)
func (t Theme) Palette() string {
	switch t {
	case ThemeLight:
		return "caramellatte"
	case ThemeDark:
		return "coffee"
	default:
		return ""
	}
}
const DefaultTimezone = "America/New_York"
var Default = Preferences{
	Theme:      ThemeSystem,
	Timezone:   DefaultTimezone,
	DateFormat: DateDMYText,
	TimeFormat: Time12H,
	FollowTurn: true,
	ShowBlood:  true,
	PingVolume: PingVolumeMax,
}
const (
	PingVolumeMax  = 100
	PingVolumeStep = 10
)
type Preferences struct {
	Theme      Theme
	Timezone   string
	DateFormat DateFormat
	TimeFormat TimeFormat
	FollowTurn bool
	ShowBlood bool
	PingVolume int
}
func New(theme, timezone, dateFormat, timeFormat string, followTurn, showBlood bool, pingVolume int) Preferences {
	p := Default
	if v, ok := ParseTheme(theme); ok {
		p.Theme = v
	}
	if v, ok := ParseTimezone(timezone); ok {
		p.Timezone = v
	}
	if v, ok := ParseDateFormat(dateFormat); ok {
		p.DateFormat = v
	}
	if v, ok := ParseTimeFormat(timeFormat); ok {
		p.TimeFormat = v
	}
	p.FollowTurn = followTurn
	p.ShowBlood = showBlood
	p.PingVolume = ClampPingVolume(pingVolume)
	return p
}
func ClampPingVolume(v int) int {
	if v < 0 {
		return 0
	}
	if v > PingVolumeMax {
		return PingVolumeMax
	}
	return v
}
func Themes() []Theme { return []Theme{ThemeSystem, ThemeLight, ThemeDark} }
func DateFormats() []DateFormat {
	return []DateFormat{DateDMYText, DateMDYText, DateMDYSlash, DateDMYSlash, DateISO}
}
func TimeFormats() []TimeFormat { return []TimeFormat{Time12H, Time24H} }
func ParseTheme(s string) (Theme, bool) {
	for _, t := range Themes() {
		if string(t) == s {
			return t, true
		}
	}
	return Default.Theme, false
}
func ParseDateFormat(s string) (DateFormat, bool) {
	for _, d := range DateFormats() {
		if string(d) == s {
			return d, true
		}
	}
	return Default.DateFormat, false
}
func ParseTimeFormat(s string) (TimeFormat, bool) {
	for _, t := range TimeFormats() {
		if string(t) == s {
			return t, true
		}
	}
	return Default.TimeFormat, false
}
func ParsePingVolume(s string) (int, bool) {
	if s == "" {
		return Default.PingVolume, true
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > PingVolumeMax || v%PingVolumeStep != 0 {
		return Default.PingVolume, false
	}
	return v, true
}
func (d DateFormat) Layout() string {
	switch d {
	case DateMDYText:
		return "Jan 2, 2006"
	case DateMDYSlash:
		return "01/02/2006"
	case DateDMYSlash:
		return "02/01/2006"
	case DateISO:
		return "2006-01-02"
	default:
		return "2 Jan 2006"
	}
}
func (d DateFormat) Format(at time.Time) string {
	return at.Format(d.Layout())
}
func (t TimeFormat) Layout() string {
	if t == Time24H {
		return "15:04"
	}
	return "3:04 PM"
}
func (p Preferences) Location() *time.Location {
	if loc, ok := zone(p.Timezone); ok {
		return loc
	}
	if loc, ok := zone(DefaultTimezone); ok {
		return loc
	}
	return time.UTC
}
func (p Preferences) Format(at time.Time) (iso string, text string) {
	layout := p.DateFormat.Layout() + ", " + p.TimeFormat.Layout() + " MST"
	return at.UTC().Format(time.RFC3339), at.In(p.Location()).Format(layout)
}
