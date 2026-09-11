package room_test

import (
	"strings"
	"testing"

	"tabletopper/internal/room"
)





const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"





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

	
	
	
	if len(seen) != len(alphabet) {
		t.Errorf("a thousand draws produced %d of the %d characters", len(seen), len(alphabet))
	}
}



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



func TestNormalizingMakesATypedCodeValid(t *testing.T) {
	typed := "  ab2c "

	if room.ValidCode(typed) {
		t.Error("ValidCode accepted an untrimmed lower-case code; it expects a normalised one")
	}
	if !room.ValidCode(room.NormalizeCode(typed)) {
		t.Errorf("ValidCode refused %q after normalising", typed)
	}
}
