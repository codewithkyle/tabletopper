package snippet

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type Match struct {
	Before string
	Match  string
	After  string
}

func Find(text, term string, radius int) (Match, bool) {
	folded, offsets := fold(text)
	needle, _ := fold(term)
	if needle == "" {
		return Match{}, false
	}
	at := strings.Index(folded, needle)
	if at < 0 {
		return Match{}, false
	}
	start, end := offsets[at], offsets[at+len(needle)]
	before, cutBefore := tail(text[:start], radius)
	after, cutAfter := head(text[end:], radius)
	if cutBefore {
		before = "…" + before
	}
	if cutAfter {
		after += "…"
	}
	return Match{Before: before, Match: text[start:end], After: after}, true
}
func Contains(text, term string) bool {
	folded, _ := fold(text)
	needle, _ := fold(term)
	return needle != "" && strings.Contains(folded, needle)
}

const wordSearch = 12

func tail(s string, radius int) (string, bool) {
	if utf8.RuneCountInString(s) <= radius {
		return s, false
	}
	cut := len(s)
	for n := 0; n < radius && cut > 0; n++ {
		_, size := utf8.DecodeLastRuneInString(s[:cut])
		cut -= size
	}
	for n := 0; n < wordSearch && cut < len(s); n++ {
		r, size := utf8.DecodeRuneInString(s[cut:])
		if unicode.IsSpace(r) {
			cut += size
			break
		}
		cut += size
	}
	return s[cut:], true
}
func head(s string, radius int) (string, bool) {
	if utf8.RuneCountInString(s) <= radius {
		return s, false
	}
	cut := 0
	for n := 0; n < radius && cut < len(s); n++ {
		_, size := utf8.DecodeRuneInString(s[cut:])
		cut += size
	}
	for n := 0; n < wordSearch && cut > 0; n++ {
		r, size := utf8.DecodeLastRuneInString(s[:cut])
		if unicode.IsSpace(r) {
			cut -= size
			break
		}
		cut -= size
	}
	return s[:cut], true
}
func fold(s string) (string, []int) {
	var out strings.Builder
	out.Grow(len(s))
	offsets := make([]int, 0, len(s)+1)
	for i, r := range s {
		if r < utf8.RuneSelf {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			out.WriteByte(byte(r))
			offsets = append(offsets, i)
			continue
		}
		for _, f := range foldRune(r) {
			for n := 0; n < utf8.RuneLen(f); n++ {
				offsets = append(offsets, i)
			}
			out.WriteRune(f)
		}
	}
	return out.String(), append(offsets, len(s))
}
func foldRune(r rune) []rune {
	decomposed := norm.NFD.String(string(r))
	out := make([]rune, 0, len(decomposed))
	for _, d := range decomposed {
		if unicode.Is(unicode.Mn, d) {
			continue
		}
		out = append(out, unicode.ToLower(d))
	}
	return out
}
