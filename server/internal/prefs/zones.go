package prefs

import "time"

type Zone struct {
	Name  string
	Label string
	Alias string
}
type ZoneGroup struct {
	Region string
	Zones  []Zone
}

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
var zones = func() map[string]*time.Location {
	m := make(map[string]*time.Location)
	for _, group := range ZoneGroups {
		for _, z := range group.Zones {
			loc, err := time.LoadLocation(z.Name)
			if err != nil {
				continue
			}
			m[z.Name] = loc
		}
	}
	return m
}()

func zone(name string) (*time.Location, bool) {
	loc, ok := zones[name]
	return loc, ok
}
func ParseTimezone(name string) (string, bool) {
	if _, ok := zones[name]; ok {
		return name, true
	}
	return Default.Timezone, false
}
