package pages
const noMatchHint = "Try a shorter term, or clear the search box to see them all."
func noMatchHeading(plural string, query string) string {
	if query == "" {
		return ""
	}
	return "No " + plural + " match \"" + query + "\"."
}
