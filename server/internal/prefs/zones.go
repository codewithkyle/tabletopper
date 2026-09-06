package prefs

import "time"

// The zone picker, and the only list of zone names in the app.
//
// IT IS CURATED BECAUSE GO OFFERS NO WAY TO ENUMERATE ONE. time/tzdata is a
// lookup table, not a directory: LoadLocation answers for any name the IANA
// database holds, and nothing exposes what those names are. Reading
// /usr/share/zoneinfo would work on a developer machine and find nothing in
// the alpine image, which is the failure the tzdata import exists to avoid.
//
// So this is a hand-written list of about fifty, which is a select a person can
// read, against the six hundred-odd names tzdata carries -- most of them
// aliases, historical spellings, or islands with one settlement. A reader whose
// zone is missing is one line away from having it, and offering the full set in
// an unsearchable <select> would serve them worse.
//
// A LABEL IS NOT DERIVED FROM THE NAME. Splitting on the slash gives
// "Argentina/Buenos_Aires" the wrong half and every other name an underscore,
// and there is nothing in "Asia/Kolkata" that says India. The names are also
// what carries the DST rules -- Phoenix and Denver are the same offset for four
// months a year and different for the other eight -- so the pairs here are not
// interchangeable with an offset list.
//
// The groups are the order the dialog shows, roughly west to east within each,
// so scanning for a city works the way a map does.

// Zone is one IANA name and what to call it in the picker.
type Zone struct {
	Name  string
	Label string
}

// ZoneGroup is one <optgroup>.
type ZoneGroup struct {
	Region string
	Zones  []Zone
}

// ZoneGroups is the offered list. Adding a zone is a line here and nothing
// else: the lookup table below is built from it, and the test that every entry
// resolves walks it.
var ZoneGroups = []ZoneGroup{
	{Region: "Universal", Zones: []Zone{
		{Name: "UTC", Label: "UTC"},
	}},
	{Region: "Americas", Zones: []Zone{
		{Name: "Pacific/Honolulu", Label: "Honolulu"},
		{Name: "America/Anchorage", Label: "Anchorage"},
		{Name: "America/Los_Angeles", Label: "Los Angeles"},
		{Name: "America/Vancouver", Label: "Vancouver"},
		{Name: "America/Phoenix", Label: "Phoenix"},
		{Name: "America/Denver", Label: "Denver"},
		{Name: "America/Chicago", Label: "Chicago"},
		{Name: "America/Mexico_City", Label: "Mexico City"},
		{Name: "America/New_York", Label: "New York"},
		{Name: "America/Toronto", Label: "Toronto"},
		{Name: "America/Bogota", Label: "Bogota"},
		{Name: "America/Halifax", Label: "Halifax"},
		{Name: "America/Sao_Paulo", Label: "Sao Paulo"},
		{Name: "America/Argentina/Buenos_Aires", Label: "Buenos Aires"},
	}},
	{Region: "Europe & Africa", Zones: []Zone{
		{Name: "Atlantic/Reykjavik", Label: "Reykjavik"},
		{Name: "Europe/Lisbon", Label: "Lisbon"},
		{Name: "Europe/Dublin", Label: "Dublin"},
		{Name: "Europe/London", Label: "London"},
		{Name: "Africa/Lagos", Label: "Lagos"},
		{Name: "Europe/Madrid", Label: "Madrid"},
		{Name: "Europe/Paris", Label: "Paris"},
		{Name: "Europe/Berlin", Label: "Berlin"},
		{Name: "Europe/Rome", Label: "Rome"},
		{Name: "Europe/Warsaw", Label: "Warsaw"},
		{Name: "Africa/Cairo", Label: "Cairo"},
		{Name: "Africa/Johannesburg", Label: "Johannesburg"},
		{Name: "Europe/Athens", Label: "Athens"},
		{Name: "Europe/Helsinki", Label: "Helsinki"},
		{Name: "Europe/Kyiv", Label: "Kyiv"},
		{Name: "Africa/Nairobi", Label: "Nairobi"},
		{Name: "Europe/Istanbul", Label: "Istanbul"},
		{Name: "Europe/Moscow", Label: "Moscow"},
	}},
	{Region: "Asia & Pacific", Zones: []Zone{
		{Name: "Asia/Jerusalem", Label: "Jerusalem"},
		{Name: "Asia/Dubai", Label: "Dubai"},
		{Name: "Asia/Karachi", Label: "Karachi"},
		{Name: "Asia/Kolkata", Label: "Kolkata"},
		{Name: "Asia/Kathmandu", Label: "Kathmandu"},
		{Name: "Asia/Dhaka", Label: "Dhaka"},
		{Name: "Asia/Bangkok", Label: "Bangkok"},
		{Name: "Asia/Jakarta", Label: "Jakarta"},
		{Name: "Asia/Singapore", Label: "Singapore"},
		{Name: "Asia/Shanghai", Label: "Shanghai"},
		{Name: "Asia/Hong_Kong", Label: "Hong Kong"},
		{Name: "Asia/Manila", Label: "Manila"},
		{Name: "Australia/Perth", Label: "Perth"},
		{Name: "Asia/Seoul", Label: "Seoul"},
		{Name: "Asia/Tokyo", Label: "Tokyo"},
		{Name: "Australia/Adelaide", Label: "Adelaide"},
		{Name: "Australia/Brisbane", Label: "Brisbane"},
		{Name: "Australia/Sydney", Label: "Sydney"},
		{Name: "Pacific/Auckland", Label: "Auckland"},
	}},
}

// zones is the list flattened into the three things every caller wants: is this
// name offered, and what location does it mean.
//
// EVERY LOCATION IS LOADED ONCE, HERE. time.LoadLocation does not cache -- it
// re-reads and re-parses the zone file on every call -- and Format runs on
// every timestamp on every page. Loading at init also means a name this build
// cannot resolve is caught by the test below rather than by a reader.
var zones = func() map[string]*time.Location {
	m := make(map[string]*time.Location)
	for _, group := range ZoneGroups {
		for _, z := range group.Zones {
			loc, err := time.LoadLocation(z.Name)
			if err != nil {
				// Left out of the map rather than panicking. A typo here is a
				// failing test in this package; on the remote chance one
				// reaches a running server, an unresolvable name falls back to
				// the default zone and the app keeps serving pages.
				continue
			}
			m[z.Name] = loc
		}
	}
	return m
}()

// zone resolves an offered name to its location.
func zone(name string) (*time.Location, bool) {
	loc, ok := zones[name]
	return loc, ok
}

// ParseTimezone reports whether a name is one this app offers, which is what
// the settings form validates against.
//
// THE CURATED LIST IS THE ALLOWLIST, not LoadLocation. Accepting any name
// tzdata happens to know would let the column fill with spellings the picker
// cannot show -- so a reader who saved one would open the dialog and find
// nothing selected, and changing anything else would silently move them.
func ParseTimezone(name string) (string, bool) {
	if _, ok := zones[name]; ok {
		return name, true
	}
	return Default.Timezone, false
}
