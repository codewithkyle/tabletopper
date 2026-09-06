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
// So this is a hand-written list of about eighty, which is a select a person
// can read, against the six hundred-odd names tzdata carries -- most of them
// aliases, historical spellings, or islands with one settlement. A reader whose
// zone is missing is one line away from having it, and offering the full set in
// an unsearchable <select> would serve them worse.
//
// LENGTH IS A UX DECISION AND IT CHANGED ONCE. The list started at about fifty
// and grew when <zone-detect> arrived: the welcome dialog preselects whatever
// Intl reports, and a name this list does not hold is a reader who silently
// gets New York on the one screen built to stop that happening. So the entries
// that were added are the ones detection was most likely to land on and miss --
// the rest of Canada, the smaller European capitals, south and southeast Asia,
// the other Australian zones -- rather than a sweep for completeness.
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

	// Alias is the older IANA spelling of this zone, where one exists, and it
	// is here for the welcome dialog's browser detection rather than for
	// storage -- nothing is ever written to the column under this name.
	//
	// IANA RENAMES ZONES AND KEEPS THE OLD NAME AS A LINK, and ICU -- which is
	// what Intl.DateTimeFormat().resolvedOptions().timeZone goes through --
	// still canonicalises several of them to the pre-rename spelling. Measured
	// against this list rather than guessed: five of the eighty come back
	// under a different name, and one of them is India.
	//
	//   Asia/Kolkata                    -> Asia/Calcutta
	//   Asia/Ho_Chi_Minh                -> Asia/Saigon
	//   Asia/Kathmandu                  -> Asia/Katmandu
	//   Europe/Kyiv                     -> Europe/Kiev
	//   America/Argentina/Buenos_Aires  -> America/Buenos_Aires
	//
	// Without these the detection silently misses every reader in those places,
	// on the one screen that exists to stop exactly that happening. Engines
	// differ and change, so both spellings are matched rather than picking
	// whichever one is currently right.
	Alias string
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
		{Name: "America/Edmonton", Label: "Edmonton"},
		{Name: "America/Phoenix", Label: "Phoenix"},
		{Name: "America/Denver", Label: "Denver"},
		{Name: "America/Chicago", Label: "Chicago"},
		{Name: "America/Winnipeg", Label: "Winnipeg"},
		{Name: "America/Guatemala", Label: "Guatemala City"},
		{Name: "America/Mexico_City", Label: "Mexico City"},
		{Name: "America/New_York", Label: "New York"},
		{Name: "America/Toronto", Label: "Toronto"},
		{Name: "America/Bogota", Label: "Bogota"},
		{Name: "America/Lima", Label: "Lima"},
		{Name: "America/Caracas", Label: "Caracas"},
		{Name: "America/Halifax", Label: "Halifax"},
		{Name: "America/Santiago", Label: "Santiago"},
		{Name: "America/St_Johns", Label: "St. John's"},
		{Name: "America/Sao_Paulo", Label: "Sao Paulo"},
		{Name: "America/Argentina/Buenos_Aires", Label: "Buenos Aires", Alias: "America/Buenos_Aires"},
	}},
	{Region: "Europe & Africa", Zones: []Zone{
		{Name: "Atlantic/Reykjavik", Label: "Reykjavik"},
		{Name: "Europe/Lisbon", Label: "Lisbon"},
		{Name: "Europe/Dublin", Label: "Dublin"},
		{Name: "Europe/London", Label: "London"},
		{Name: "Africa/Casablanca", Label: "Casablanca"},
		{Name: "Africa/Accra", Label: "Accra"},
		{Name: "Africa/Lagos", Label: "Lagos"},
		{Name: "Africa/Algiers", Label: "Algiers"},
		{Name: "Europe/Madrid", Label: "Madrid"},
		{Name: "Europe/Paris", Label: "Paris"},
		{Name: "Europe/Brussels", Label: "Brussels"},
		{Name: "Europe/Amsterdam", Label: "Amsterdam"},
		{Name: "Europe/Copenhagen", Label: "Copenhagen"},
		{Name: "Europe/Oslo", Label: "Oslo"},
		{Name: "Europe/Berlin", Label: "Berlin"},
		{Name: "Europe/Zurich", Label: "Zurich"},
		{Name: "Europe/Vienna", Label: "Vienna"},
		{Name: "Europe/Rome", Label: "Rome"},
		{Name: "Europe/Prague", Label: "Prague"},
		{Name: "Europe/Stockholm", Label: "Stockholm"},
		{Name: "Europe/Warsaw", Label: "Warsaw"},
		{Name: "Africa/Cairo", Label: "Cairo"},
		{Name: "Africa/Johannesburg", Label: "Johannesburg"},
		{Name: "Europe/Athens", Label: "Athens"},
		{Name: "Europe/Bucharest", Label: "Bucharest"},
		{Name: "Europe/Helsinki", Label: "Helsinki"},
		{Name: "Europe/Kyiv", Label: "Kyiv", Alias: "Europe/Kiev"},
		{Name: "Africa/Nairobi", Label: "Nairobi"},
		{Name: "Europe/Istanbul", Label: "Istanbul"},
		{Name: "Europe/Moscow", Label: "Moscow"},
	}},
	{Region: "Asia & Pacific", Zones: []Zone{
		{Name: "Asia/Jerusalem", Label: "Jerusalem"},
		{Name: "Asia/Riyadh", Label: "Riyadh"},
		{Name: "Asia/Tehran", Label: "Tehran"},
		{Name: "Asia/Dubai", Label: "Dubai"},
		{Name: "Asia/Karachi", Label: "Karachi"},
		{Name: "Asia/Kolkata", Label: "Kolkata", Alias: "Asia/Calcutta"},
		{Name: "Asia/Kathmandu", Label: "Kathmandu", Alias: "Asia/Katmandu"},
		{Name: "Asia/Colombo", Label: "Colombo"},
		{Name: "Asia/Dhaka", Label: "Dhaka"},
		{Name: "Asia/Bangkok", Label: "Bangkok"},
		{Name: "Asia/Ho_Chi_Minh", Label: "Ho Chi Minh City", Alias: "Asia/Saigon"},
		{Name: "Asia/Jakarta", Label: "Jakarta"},
		{Name: "Asia/Kuala_Lumpur", Label: "Kuala Lumpur"},
		{Name: "Asia/Singapore", Label: "Singapore"},
		{Name: "Asia/Shanghai", Label: "Shanghai"},
		{Name: "Asia/Hong_Kong", Label: "Hong Kong"},
		{Name: "Asia/Taipei", Label: "Taipei"},
		{Name: "Asia/Manila", Label: "Manila"},
		{Name: "Australia/Perth", Label: "Perth"},
		{Name: "Asia/Seoul", Label: "Seoul"},
		{Name: "Asia/Tokyo", Label: "Tokyo"},
		{Name: "Australia/Darwin", Label: "Darwin"},
		{Name: "Australia/Adelaide", Label: "Adelaide"},
		{Name: "Australia/Brisbane", Label: "Brisbane"},
		{Name: "Australia/Hobart", Label: "Hobart"},
		{Name: "Australia/Sydney", Label: "Sydney"},
		{Name: "Pacific/Fiji", Label: "Fiji"},
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
