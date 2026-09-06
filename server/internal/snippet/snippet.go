// Package snippet finds a search term inside a journal entry and cuts the
// sentence around it, so a result can show why it matched instead of only that
// it did.
//
// IT IS ALSO WHAT DECIDES A RESULT SURVIVES. SearchCharacterJournals runs LIKE
// against the stored markdown, which matches things a reader never sees -- an
// entry holding an image comes back for the term `assets` because that word is
// in the URL behind the picture. So the SQL is the candidate filter and this is
// the answer: an entry whose term is nowhere in its visible text is dropped
// rather than listed with a snippet that does not contain what was typed.
//
// MATCHING FOLDS CASE AND ACCENTS, because the column it is standing in for
// does. journals is utf8mb4_0900_ai_ci, so MySQL has already matched
// `Beornegar` against `Béornegar` by the time a row gets here -- and a stricter
// comparison in Go would drop a row the database was right to return, which the
// reader would watch happen as they typed a name the way they heard it. The
// fold is NFD, drop the nonspacing marks, lowercase.
//
// IT IS THE COLUMN'S COLLATION THAT DECIDES, NOT THE CONNECTION'S. The driver
// connects as utf8mb4_general_ci and the pattern is a literal, so on
// coercibility the column wins the comparison -- which is why the accent match
// works without the DSN naming a collation, and why making it name one is a
// change to think about rather than tidying.
//
// LIGATURES ARE THE ONE PLACE THE COLLATION GOES FURTHER, AND LIKE DOES NOT GO
// THERE EITHER. Under = the collation expands: 'Straße' = 'Strasse' and
// 'Æthelred' = 'Aethelred' both come back true. Under LIKE neither matches,
// because a pattern is walked position by position and an expansion is one
// character standing for two. The search is a LIKE, so a row whose only tie to
// the term is a ligature never arrives here to be dropped -- this fold not
// expanding them costs nothing, and teaching it to would only find rows the
// query did not return.
package snippet

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Match is one hit, split around the term so the caller can mark it. The three
// fields are plain text and are rendered as plain text -- the <mark> is markup
// the template writes, never a string built here, which is what keeps a journal
// body out of the one code path in this app that writes unescaped HTML.
//
// Match holds the text as the writer typed it rather than as it was searched
// for: someone looking for `beornegar` gets `Béornegar` marked, because what is
// highlighted is the entry, not the query.
type Match struct {
	// Before and After carry a leading and trailing ellipsis when the window
	// cut the text. The template joins the three with no separator, so any
	// space between them is a space one of them ends with.
	Before string
	Match  string
	After  string
}

// Find locates term in text and returns the window around it. radius is how
// many runes of context to keep on each side before trimming back to a word
// boundary.
//
// The FIRST match wins. An entry mentioning a town nine times is still one
// result and one line of context, and the first mention is the one most likely
// to be the sentence that introduces it.
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

	// offsets maps a byte in the folded string back to the byte in text that
	// produced it, so a window measured against the fold can be cut out of the
	// original. It carries one extra entry so a match ending at the last byte
	// has an end to read.
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

// Contains reports whether term appears in text under the same folding Find
// uses. It is what a title is checked with: a title is a plain column rather
// than markdown, so a match in one is always visible and needs no window cut
// around it.
func Contains(text, term string) bool {
	folded, _ := fold(text)
	needle, _ := fold(term)

	return needle != "" && strings.Contains(folded, needle)
}

// wordSearch is how far past the radius a cut will look for a space rather than
// landing in the middle of a word. Beyond this the word is longer than the
// context is worth and the cut is taken where it fell.
const wordSearch = 12

// tail keeps the last radius runes of s, backing up to a word boundary. The
// bool reports whether anything was dropped, which is what earns the ellipsis.
func tail(s string, radius int) (string, bool) {
	if utf8.RuneCountInString(s) <= radius {
		return s, false
	}

	cut := len(s)
	for n := 0; n < radius && cut > 0; n++ {
		_, size := utf8.DecodeLastRuneInString(s[:cut])
		cut -= size
	}

	// Forward to the next space, so the window starts at a word rather than
	// inside one. Searching forward rather than back also guarantees the
	// result is no longer than the radius asked for.
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

// head keeps the first radius runes of s, stopping at a word boundary.
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

// fold returns s case- and accent-folded, and a map from each byte of the
// result back to the byte of s it came from.
//
// THE MAP IS WHY THIS IS NOT THREE LINES OF x/text. A transform chain folds a
// whole string and loses track of where anything was, and the caller needs the
// original text to show: a snippet cut at an offset measured in the folded
// string would slice `Béornegar` in the wrong place, because é is two bytes
// there and one here. So the fold runs a rune at a time and records the origin
// of every byte it writes, plus a final entry holding len(s) so a match that
// runs to the end of the string has an end offset to read.
//
// ASCII TAKES A FAST PATH, and it is the path essentially every rune takes. The
// general case allocates a string per rune to decompose it, which would be a
// hundred thousand allocations on a long entry; below utf8.RuneSelf the fold is
// one byte in and one byte out and none of that is reached.
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

// foldRune decomposes one rune, drops the accents it separated out, and
// lowercases what is left. It can return more than one rune -- a decomposition
// that keeps two base letters -- and it can return none, for a rune that was
// nothing but a combining mark to begin with.
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
