package dnd

type Option struct {
	Label string
	Value string
}

func normalize(value string, options []Option, fallback string) string {
	for _, option := range options {
		if option.Value == value {
			return value
		}
	}
	return fallback
}
func label(value string, options []Option) string {
	for _, option := range options {
		if option.Value == value {
			return option.Label
		}
	}
	return ""
}
