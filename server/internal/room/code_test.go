package room_test

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)

// The alphabet, repeated here rather than exported, so that a change to the
// generator's own constant does not quietly change what these tests accept.
// Reading a code out over voice chat is what it is for, which is why I, L, O, 0
// and 1 are not in it.
const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// A thousand draws, because the generator rejects bytes that would bias the
// distribution and refills its buffer to do it -- a bug in that loop shows up
// as a short code or a character from outside the set, and neither is certain
// to appear in one draw.
func TestNewCodeOnlyDrawsFromTheAlphabet(t *testing.T) {
	seen := map[rune]bool{}

	for range 1000 {
		code := room.NewCode()

		if len(code) != room.CodeLength {
			t.Fatalf("NewCode() = %q, which is %d characters, want %d", code, len(code), room.CodeLength)
		}
		for _, c := range code {
			if !strings.ContainsRune(alphabet, c) {
				t.Fatalf("NewCode() = %q, which contains %q -- not in the alphabet", code, c)
			}
			seen[c] = true
		}
		if !room.ValidCode(code) {
			t.Fatalf("NewCode() = %q, which ValidCode refuses", code)
		}
	}

	// Four thousand characters over an alphabet of thirty-one: a character that
	// never came up is a generator that cannot reach part of its own range,
	// which is what a modulo applied to the wrong ceiling looks like.
	if len(seen) != len(alphabet) {
		t.Errorf("a thousand draws produced %d of the %d characters", len(seen), len(alphabet))
	}
}

// Two codes in a row being equal is a one-in-920,000 event, so this is a test
// for a generator that has stopped drawing rather than a test of randomness.
func TestNewCodeDoesNotRepeatItself(t *testing.T) {
	first := room.NewCode()

	for range 100 {
		if room.NewCode() != first {
			return
		}
	}

	t.Fatalf("a hundred draws all came back %q", first)
}

func TestValidCode(t *testing.T) {
	for name, c := range map[string]struct {
		code string
		want bool
	}{
		"a code":                {"AB2C", true},
		"every letter the same": {"ZZZZ", true},
		"all digits":            {"2345", true},
		"lower case":            {"ab2c", false},
		"three characters":      {"AB2", false},
		"five characters":       {"AB2CD", false},
		"empty":                 {"", false},
		// The five that were left out because they are misread aloud or on
		// screen. Each is refused rather than folded onto its lookalike: a
		// code with an O in it is not a code with a zero in it.
		"an I":          {"AIB2", false},
		"an L":          {"ALB2", false},
		"an O":          {"AOB2", false},
		"a zero":        {"A0B2", false},
		"a one":         {"A1B2", false},
		"a space":       {"AB 2", false},
		"punctuation":   {"AB-2", false},
		"a wide rune":   {"AB2é", false},
		"leading space": {" AB2", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := room.ValidCode(c.code); got != c.want {
				t.Errorf("ValidCode(%q) = %v, want %v", c.code, got, c.want)
			}
		})
	}
}

// A player types what they were told, which arrives in whatever case their
// keyboard was in and with whatever a copy-paste picked up around it.
func TestNormalizeCode(t *testing.T) {
	for name, c := range map[string]struct{ in, want string }{
		"lower case":        {"ab2c", "AB2C"},
		"mixed case":        {"aB2c", "AB2C"},
		"surrounding space": {"  ab2c \n", "AB2C"},
		"already normal":    {"AB2C", "AB2C"},
		"empty":             {"", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := room.NormalizeCode(c.in); got != c.want {
				t.Errorf("NormalizeCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// The pair is what the join actually runs, in that order, and the property is
// that a code a person could plausibly type gets through it.
func TestNormalizingMakesATypedCodeValid(t *testing.T) {
	typed := "  ab2c "

	if room.ValidCode(typed) {
		t.Error("ValidCode accepted an untrimmed lower-case code; it expects a normalised one")
	}
	if !room.ValidCode(room.NormalizeCode(typed)) {
		t.Errorf("ValidCode refused %q after normalising", typed)
	}
}
