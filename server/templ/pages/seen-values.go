package pages

type SeenValue struct {
	Field string
	Value string
}

func SeenName(field string) string { return "seen_" + field }
func seenID(field string) string   { return "seen-" + field }
