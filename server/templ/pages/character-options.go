package pages


type Option struct {
	Label string
	Value string
}









var alignmentOptions = []Option{
	{Label: "Unaligned", Value: "unaligned"},
	{Label: "Any Alignment", Value: "any alignment"},
	{Label: "Lawful Good", Value: "lawful good"},
	{Label: "Neutral Good", Value: "neutral good"},
	{Label: "Chaotic Good", Value: "chaotic good"},
	{Label: "Lawful Neutral", Value: "lawful neutral"},
	{Label: "Neutral", Value: "neutral"},
	{Label: "Chaotic Neutral", Value: "chaotic neutral"},
	{Label: "Lawful Evil", Value: "lawful evil"},
	{Label: "Neutral Evil", Value: "neutral evil"},
	{Label: "Chaotic Evil", Value: "chaotic evil"},
	{Label: "Any Neutral Alignment", Value: "any neutral alignment"},
	{Label: "Any Good Alignment", Value: "any good alignment"},
	{Label: "Any Chaotic Alignment", Value: "any chaotic alignment"},
	{Label: "Any Lawful Alignment", Value: "any lawful alignment"},
	{Label: "Any Non-Good Alignment", Value: "any non-good alignment"},
}

var sizeOptions = []Option{
	{Label: "Tiny", Value: "tiny"},
	{Label: "Small", Value: "small"},
	{Label: "Medium", Value: "medium"},
	{Label: "Large", Value: "large"},
	{Label: "Huge", Value: "huge"},
	{Label: "Gargantuan", Value: "gargantuan"},
}












const (
	DefaultAlignment = "unaligned"
	DefaultSize      = "medium"
)










func NormalizeSize(value string) string {
	return normalizeOption(value, sizeOptions, DefaultSize)
}

func NormalizeAlignment(value string) string {
	return normalizeOption(value, alignmentOptions, DefaultAlignment)
}

func normalizeOption(value string, options []Option, fallback string) string {
	for _, option := range options {
		if option.Value == value {
			return value
		}
	}

	return fallback
}






func SizeLabel(value string) string {
	for _, option := range sizeOptions {
		if option.Value == value {
			return option.Label
		}
	}

	return ""
}
