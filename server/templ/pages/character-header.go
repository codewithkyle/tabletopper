package pages
import "strings"
type CharacterHeader struct {
	Name string
	Subtitle string
	AvatarID string
	AC          string
	CurrentHP   string
	MaxHP       string
	Speed       string
	Initiative  string
	Proficiency string
	Passive     string
}
func characterBarName(header CharacterHeader) string {
	if strings.TrimSpace(header.Name) == "" {
		return "Unnamed character"
	}
	return header.Name
}
func AlignmentLabel(value string) string {
	for _, option := range alignmentOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return ""
}
type headerChip struct {
	Label string
	Value string
	Sub   string
}
func characterChips(header CharacterHeader) []headerChip {
	speed, speedUnit := splitMeasurement(header.Speed)
	return []headerChip{
		{Label: "AC", Value: header.AC},
		{Label: "Hit Points", Value: header.CurrentHP, Sub: "/ " + header.MaxHP},
		{Label: "Speed", Value: speed, Sub: speedUnit},
		{Label: "Initiative", Value: header.Initiative},
		{Label: "Proficiency", Value: header.Proficiency},
		{Label: "Passive", Value: header.Passive},
	}
}
func splitMeasurement(value string) (string, string) {
	value = strings.TrimSpace(value)
	digits := 0
	for digits < len(value) && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return value, ""
	}
	return value[:digits], strings.TrimSpace(value[digits:])
}
