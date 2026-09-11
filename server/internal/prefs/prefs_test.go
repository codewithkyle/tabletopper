package prefs

import (
	"strings"
	"testing"
	"time"
)





var (
	summer = time.Date(2026, 9, 6, 18, 4, 11, 0, time.UTC)
	winter = time.Date(2026, 1, 5, 18, 4, 11, 0, time.UTC)
)









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




func TestAZoneWithNoAbbreviationRendersItsOffset(t *testing.T) {
	p := Preferences{Timezone: "Asia/Kathmandu", DateFormat: DateISO, TimeFormat: Time24H}

	_, got := p.Format(summer)
	if want := "2026-09-06, 23:49 +0545"; got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}




func TestTheISOHalfIsAlwaysUTC(t *testing.T) {
	for _, tz := range []string{"UTC", "America/New_York", "Australia/Sydney", "Asia/Kathmandu"} {
		p := Preferences{Timezone: tz, DateFormat: DateISO, TimeFormat: Time24H}

		iso, _ := p.Format(summer)
		if want := "2026-09-06T18:04:11Z"; iso != want {
			t.Errorf("Format() iso for %s = %q, want %q", tz, iso, want)
		}
	}
}



func TestTheZeroValueStillRenders(t *testing.T) {
	var p Preferences

	if p.Location() == nil {
		t.Fatal("Location() is nil for the zero value")
	}

	iso, text := p.Format(summer)
	if iso == "" || text == "" {
		t.Fatalf("Format() = %q, %q; both should be rendered", iso, text)
	}
	
	
	if _, want := Default.Format(summer); text != want {
		t.Errorf("Format() = %q, want the default rendering %q", text, want)
	}
}



func TestNewFallsBackFieldByField(t *testing.T) {
	p := New("dark", "nonsense/Nowhere", "iso", "", true, true, PingVolumeMax)

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
	if !p.FollowTurn {
		t.Error("FollowTurn = false, want the stored true: a boolean has nothing to fall back to")
	}
	if !p.ShowBlood {
		t.Error("ShowBlood = false, want the stored true: a boolean has nothing to fall back to")
	}
}









func TestTheCameraFollowsTheTurnUntilSomebodySaysOtherwise(t *testing.T) {
	if !Default.FollowTurn {
		t.Error("Default.FollowTurn = false, want true")
	}
	if p := New("", "", "", "", false, true, PingVolumeMax); p.FollowTurn {
		t.Error("New ignored a stored false")
	}
	if (Preferences{}).FollowTurn {
		t.Error("the zero value has it on; the note on the field is wrong")
	}
}





func TestTheFloorTakesBloodUntilSomebodySaysOtherwise(t *testing.T) {
	if !Default.ShowBlood {
		t.Error("Default.ShowBlood = false, want true")
	}
	if p := New("", "", "", "", true, false, PingVolumeMax); p.ShowBlood {
		t.Error("New ignored a stored false")
	}
	if (Preferences{}).ShowBlood {
		t.Error("the zero value has it on; the note on the field is wrong")
	}
}





func TestTheTwoTableSettingsAreNotEachOther(t *testing.T) {
	if p := New("", "", "", "", true, false, PingVolumeMax); !p.FollowTurn || p.ShowBlood {
		t.Errorf("New(followTurn: true, showBlood: false) = %+v", p)
	}
	if p := New("", "", "", "", false, true, PingVolumeMax); p.FollowTurn || !p.ShowBlood {
		t.Errorf("New(followTurn: false, showBlood: true) = %+v", p)
	}
}



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
	
	
	if _, text := Default.Format(summer); !strings.Contains(text, "Sep") {
		t.Errorf("the default rendering %q does not spell its month", text)
	}
}








func TestEveryAliasIsTheSameZoneUnderItsOldName(t *testing.T) {
	seen := map[string]string{}

	for _, group := range ZoneGroups {
		for _, z := range group.Zones {
			if z.Alias == "" {
				continue
			}

			if _, offered := zones[z.Alias]; offered {
				t.Errorf("%q is both an alias of %q and an offered zone in its own right", z.Alias, z.Name)
			}
			if other, dup := seen[z.Alias]; dup {
				t.Errorf("alias %q hangs off both %q and %q", z.Alias, other, z.Name)
			}
			seen[z.Alias] = z.Name

			old, err := time.LoadLocation(z.Alias)
			if err != nil {
				t.Errorf("alias %q does not resolve: %v", z.Alias, err)
				continue
			}
			current, ok := zone(z.Name)
			if !ok {
				t.Errorf("zone %q does not resolve", z.Name)
				continue
			}

			for _, at := range []time.Time{summer, winter} {
				_, want := at.In(current).Zone()
				_, got := at.In(old).Zone()
				if got != want {
					t.Errorf("%q and its alias %q disagree at %s: %d vs %d seconds",
						z.Name, z.Alias, at, want, got)
				}
			}
		}
	}

	
	
	
	if len(seen) != 5 {
		t.Errorf("%d aliases, want the 5 that ICU still canonicalises", len(seen))
	}
}
