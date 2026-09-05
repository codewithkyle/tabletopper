package pages

import (
	"database/sql"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Presentation helpers for the character cards. They live in a .go file
// rather than inside characters.templ so gofmt and go vet see them.

// characterValueOrFallback renders an optional text column, standing in
// fallback for NULL and for whitespace.
func characterValueOrFallback(value sql.NullString, fallback string) string {
	if trimmed := strings.TrimSpace(value.String); value.Valid && trimmed != "" {
		return trimmed
	}
	return fallback
}

// characterInitial is the placeholder portrait: the first letter of the name.
func characterInitial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	first, _ := utf8.DecodeRuneInString(name)
	return strings.ToUpper(string(first))
}

func signedNumber(value int) string {
	if value > 0 {
		return "+" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

// formatCharacterSize capitalises the stored lower-case size for display.
func formatCharacterSize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "Medium"
	}
	first, width := utf8.DecodeRuneInString(value)
	return strings.ToUpper(string(first)) + value[width:]
}

func characterSpeedOrFallback(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "30 ft."
}
