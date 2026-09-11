package pages

import (
	"database/sql"
	"strconv"
	"strings"
	"unicode/utf8"
)






func characterValueOrFallback(value sql.NullString, fallback string) string {
	if trimmed := strings.TrimSpace(value.String); value.Valid && trimmed != "" {
		return trimmed
	}
	return fallback
}


func characterInitial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	first, _ := utf8.DecodeRuneInString(name)
	return strings.ToUpper(string(first))
}










func SignedNumber(value int) string {
	if value >= 0 {
		return "+" + strconv.Itoa(value)
	}

	return strconv.Itoa(value)
}


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
