package pages

const noMatchHint = "Try a shorter term, or clear the search box to see them all."

func noMatchHeading(plural string, query string) string {
	return noMatchFor(plural, "match", query)
}
func noMatchFor(subject string, verb string, query string) string {
	if query == "" {
		return ""
	}
	return "No " + subject + " " + verb + " \"" + query + "\"."
}
