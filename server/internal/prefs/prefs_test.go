package prefs

import (
	"strings"
	"testing"
	"time"
)

// summer and winter are the same wall clock in New York on either side of a DST
// boundary, which is what makes them a pair worth having: 18:04 UTC is 14:04
// EDT in September and 13:04 EST in January. A format test that only ever ran
// in one of them would pass with the offset hard-coded.
var (
	summer = time.Date(2026, 9, 6, 18, 4, 11, 0, time.UTC)
	winter = time.Date(2026, 1, 5, 18, 4, 11, 0, time.UTC)
)

// EVERY OFFERED ZONE HAS TO RESOLVE, and this is the test the curated list
// exists to be checkable by. A typo in zones.go is otherwise invisible: the
// name is dropped from the map at init, the reader who picks it silently gets
// the default zone, and every date they read is quietly wrong.
//
// It is also the test that catches tzdata moving underneath us. Europe/Kyiv is
// a 2022 spelling and Europe/Kiev is what came before it; a Go release that
// dropped one would fail here rather than in production.
func TestEveryOfferedZoneResolves(t *testing.T) {
	for _, group := range ZoneGroups {
		if group.Region == "" {
			t.Error("a zone group has no region name")
		}
		if len(group.Zones) == 0 {
			t.Errorf("zone group %q offers nothing", group.Region)
		}

		for _, z := range group.Zones {
			if z.Label == "" {
				t.Errorf("zone %q has no label", z.Name)
			}
			if _, ok := zone(z.Name); !ok {
				t.Errorf("zone %q does not resolve; it would fall back to %s", z.Name, DefaultTimezone)
			}
		}
	}
}

// A name offered twice renders two identical options, and whichever the reader
// picks the other looks unselected.
func TestNoZoneIsOfferedTwice(t *testing.T) {
	seen := map[string]string{}
	for _, group := range ZoneGroups {
		for _, z := range group.Zones {
			if where, dup := seen[z.Name]; dup {
				t.Errorf("zone %q is in both %q and %q", z.Name, where, group.Region)
			}
			seen[z.Name] = group.Region
		}
	}
}

// The default zone has to be one of the offered ones, or the dialog opens with
// nothing selected for every user who has never saved.
func TestTheDefaultZoneIsOffered(t *testing.T) {
	if _, ok := ParseTimezone(DefaultTimezone); !ok {
		t.Fatalf("the default zone %q is not in the picker", DefaultTimezone)
	}
}

func TestEveryFormatCombinationRenders(t *testing.T) {
	tests := []struct {
		name string
		at   time.Time
		p    Preferences
		want string
	}{
		{
			name: "the default: unambiguous day-first, twelve hour, New York in summer",
			at:   summer,
			p:    Default,
			want: "6 Sep 2026, 2:04 PM EDT",
		},
		{
			name: "the same preferences in winter pick up the other abbreviation",
			at:   winter,
			p:    Default,
			want: "5 Jan 2026, 1:04 PM EST",
		},
		{
			name: "month-first textual",
			at:   summer,
			p:    Preferences{Timezone: "America/New_York", DateFormat: DateMDYText, TimeFormat: Time12H},
			want: "Sep 6, 2026, 2:04 PM EDT",
		},
		{
			name: "month-first numeric is zero padded",
			at:   summer,
			p:    Preferences{Timezone: "America/New_York", DateFormat: DateMDYSlash, TimeFormat: Time12H},
			want: "09/06/2026, 2:04 PM EDT",
		},
		{
			name: "day-first numeric puts the same two numbers the other way round",
			at:   summer,
			p:    Preferences{Timezone: "America/New_York", DateFormat: DateDMYSlash, TimeFormat: Time12H},
			want: "06/09/2026, 2:04 PM EDT",
		},
		{
			name: "iso with a twenty-four hour clock",
			at:   summer,
			p:    Preferences{Timezone: "America/New_York", DateFormat: DateISO, TimeFormat: Time24H},
			want: "2026-09-06, 14:04 EDT",
		},
		{
			name: "London in summer is an hour ahead of UTC",
			at:   summer,
			p:    Preferences{Timezone: "Europe/London", DateFormat: DateDMYText, TimeFormat: Time24H},
			want: "6 Sep 2026, 19:04 BST",
		},
		{
			name: "UTC renders as UTC and shifts nothing",
			at:   summer,
			p:    Preferences{Timezone: "UTC", DateFormat: DateDMYText, TimeFormat: Time24H},
			want: "6 Sep 2026, 18:04 UTC",
		},
		{
			name: "Sydney in September is the next day already",
			at:   summer,
			p:    Preferences{Timezone: "Australia/Sydney", DateFormat: DateDMYSlash, TimeFormat: Time24H},
			want: "07/09/2026, 04:04 AEST",
		},
		{
			name: "midnight reads twelve, not zero, on a twelve hour clock",
			at:   time.Date(2026, 9, 6, 4, 30, 0, 0, time.UTC),
			p:    Preferences{Timezone: "America/New_York", DateFormat: DateISO, TimeFormat: Time12H},
			want: "2026-09-06, 12:30 AM EDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := tt.p.Format(tt.at)
			if got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

// A ZONE ABBREVIATION IS NOT ALWAYS LETTERS. Kathmandu has none, so Go renders
// the offset instead. This is in the list on purpose -- it is the case that
// breaks anything written to expect three capitals.
func TestAZoneWithNoAbbreviationRendersItsOffset(t *testing.T) {
	p := Preferences{Timezone: "Asia/Kathmandu", DateFormat: DateISO, TimeFormat: Time24H}

	_, got := p.Format(summer)
	if want := "2026-09-06, 23:49 +0545"; got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

// THE MACHINE-READABLE HALF IGNORES ALL FOUR SETTINGS. It is the instant, and
// an instant is the same everywhere; a datetime attribute that moved with the
// reader's zone would be a value nobody could compare.
func TestTheISOHalfIsAlwaysUTC(t *testing.T) {
	for _, tz := range []string{"UTC", "America/New_York", "Australia/Sydney", "Asia/Kathmandu"} {
		p := Preferences{Timezone: tz, DateFormat: DateISO, TimeFormat: Time24H}

		iso, _ := p.Format(summer)
		if want := "2026-09-06T18:04:11Z"; iso != want {
			t.Errorf("Format() iso for %s = %q, want %q", tz, iso, want)
		}
	}
}

// The zero value arrives from a signed-out request and from any component
// rendered in a test, so it has to render rather than panic on a nil location.
func TestTheZeroValueStillRenders(t *testing.T) {
	var p Preferences

	if p.Location() == nil {
		t.Fatal("Location() is nil for the zero value")
	}

	iso, text := p.Format(summer)
	if iso == "" || text == "" {
		t.Fatalf("Format() = %q, %q; both should be rendered", iso, text)
	}
	// Falling back field by field means the zero value renders exactly as a
	// user who has never opened the dialog does.
	if _, want := Default.Format(summer); text != want {
		t.Errorf("Format() = %q, want the default rendering %q", text, want)
	}
}

// New is the read path and never fails: a column holding something this build
// does not know about should cost one wrong field, not a blank page.
func TestNewFallsBackFieldByField(t *testing.T) {
	p := New("dark", "nonsense/Nowhere", "iso", "")

	if p.Theme != ThemeDark {
		t.Errorf("Theme = %q, want %q", p.Theme, ThemeDark)
	}
	if p.Timezone != DefaultTimezone {
		t.Errorf("Timezone = %q, want the default %q", p.Timezone, DefaultTimezone)
	}
	if p.DateFormat != DateISO {
		t.Errorf("DateFormat = %q, want %q", p.DateFormat, DateISO)
	}
	if p.TimeFormat != Default.TimeFormat {
		t.Errorf("TimeFormat = %q, want the default %q", p.TimeFormat, Default.TimeFormat)
	}
}

// The write path does report the difference, which is what lets the form say so
// instead of storing something the picker cannot show.
func TestTheParsersRefuseWhatIsNotOffered(t *testing.T) {
	if _, ok := ParseTheme("caramellatte"); ok {
		t.Error("ParseTheme accepted a DaisyUI theme name; the column stores intent, not a palette")
	}
	if _, ok := ParseTheme(""); ok {
		t.Error("ParseTheme accepted an empty string")
	}
	if _, ok := ParseDateFormat("02/01/2006"); ok {
		t.Error("ParseDateFormat accepted a Go layout string")
	}
	if _, ok := ParseTimeFormat("24"); ok {
		t.Error("ParseTimeFormat accepted a near miss")
	}
	// A real IANA name that this app does not offer is still refused, because
	// the picker is the allowlist rather than tzdata.
	if _, err := time.LoadLocation("America/Nipigon"); err == nil {
		if _, ok := ParseTimezone("America/Nipigon"); ok {
			t.Error("ParseTimezone accepted a zone the picker does not list")
		}
	}
	for _, s := range []string{"", "UTC+5", "Etc/GMT-3", "../../etc/passwd"} {
		if _, ok := ParseTimezone(s); ok {
			t.Errorf("ParseTimezone accepted %q", s)
		}
	}
}

// Every member of every ordered list has to round-trip through its own parser,
// which is what keeps these lists in step with the ENUM members they mirror.
func TestEveryOfferedMemberParses(t *testing.T) {
	for _, v := range Themes() {
		if got, ok := ParseTheme(string(v)); !ok || got != v {
			t.Errorf("ParseTheme(%q) = %q, %v", v, got, ok)
		}
	}
	for _, v := range DateFormats() {
		if got, ok := ParseDateFormat(string(v)); !ok || got != v {
			t.Errorf("ParseDateFormat(%q) = %q, %v", v, got, ok)
		}
		// Each has to render something, and no two may render alike -- an
		// option list with a repeat in it is a choice that does nothing.
		if v.Format(summer) == "" {
			t.Errorf("DateFormat %q renders nothing", v)
		}
	}
	for _, v := range TimeFormats() {
		if got, ok := ParseTimeFormat(string(v)); !ok || got != v {
			t.Errorf("ParseTimeFormat(%q) = %q, %v", v, got, ok)
		}
	}

	seen := map[string]DateFormat{}
	for _, v := range DateFormats() {
		rendered := v.Format(summer)
		if other, dup := seen[rendered]; dup {
			t.Errorf("DateFormat %q and %q both render %q", other, v, rendered)
		}
		seen[rendered] = v
	}
}

// The defaults are a decision and not an accident, so they are written down
// where changing one breaks a test rather than only changing what everybody
// sees.
func TestTheDefaultsAreWhatWasAgreed(t *testing.T) {
	if Default.Theme != ThemeSystem {
		t.Errorf("default theme = %q, want %q", Default.Theme, ThemeSystem)
	}
	if Default.Timezone != "America/New_York" {
		t.Errorf("default timezone = %q", Default.Timezone)
	}
	if Default.DateFormat != DateDMYText {
		t.Errorf("default date format = %q, want the unambiguous one", Default.DateFormat)
	}
	if Default.TimeFormat != Time12H {
		t.Errorf("default time format = %q", Default.TimeFormat)
	}
	// The default rendering names its month rather than numbering it, which is
	// the property that makes it safe for a reader who has chosen nothing.
	if _, text := Default.Format(summer); !strings.Contains(text, "Sep") {
		t.Errorf("the default rendering %q does not spell its month", text)
	}
}
