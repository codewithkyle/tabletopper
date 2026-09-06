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

// SignedNumber prints a bonus the way a sheet does: +3, +0, -1. Exported because
// every derived bonus on the character page is formatted in the controller and
// arrives here as a string.
//
// ZERO CARRIES THE PLUS. It used to print a bare "0", which was invisible while
// the only caller was the proficiency bonus -- that is never zero. A derived
// bonus is zero all the time, and a column reading "+3 / 0 / -1" reads as though
// the middle one is a different kind of number rather than a modifier that
// happens to add nothing.
func SignedNumber(value int) string {
	if value >= 0 {
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
