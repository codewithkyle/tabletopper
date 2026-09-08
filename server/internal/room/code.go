// Package room is the virtual tabletop's own logic, kept out of the handlers so
// that it is testable without a socket or a request. It starts with the two
// things the room shell needs and nothing else: the four-character code that
// gets a player to a table, and the role that says what they may do once they
// are there.
package room

import (
	"crypto/rand"
	"strconv"
	"strings"
)

// CodeLength is four characters. A code is read aloud over voice chat and typed
// by somebody who is already annoyed that the game has not started, so it is as
// short as it can be while still being unguessable in bulk: four characters of
// the alphabet below is about 920,000 codes, against a handful open at once, so
// a guess lands on a live room roughly never -- and the join endpoint is rate
// limited per user on top of that.
const CodeLength = 4

// codeAlphabet omits I, L, O, 0 and 1, which are the pairs a person reading a
// code off a screen to a friend gets wrong. What is left is 31 characters, all
// upper case, so there is no case to mishear either.
//
// THE COUNT MATTERS TO NewCode. 31 does not divide 256, so a byte taken modulo
// the length would draw the first eight letters slightly more often than the
// rest. The rejection below is what removes that bias; changing this string
// changes the threshold it computes.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// codeCeiling is the largest multiple of the alphabet's length that fits in a
// byte. A byte at or above it is thrown away rather than folded in, which is
// what makes every letter equally likely.
const codeCeiling = 256 - 256%len(codeAlphabet)

// CodePattern is the code's shape written as an HTML pattern attribute, so the
// field on the join page cannot drift from the alphabet above.
//
// IT CARRIES BOTH CASES because the pattern attribute has no case-insensitive
// flag and the field accepts what the player's keyboard was in -- the same
// difference NormalizeCode settles on the way to the database. Duplicating the
// digits across the two halves is harmless: a character class is a set.
//
// The browser refusing a malformed code is a convenience and not the check.
// ValidCode is the check, it runs before any statement, and it runs against a
// value that may not have come from this field at all.
func CodePattern() string {
	return "[" + codeAlphabet + strings.ToLower(codeAlphabet) + "]{" + strconv.Itoa(CodeLength) + "}"
}

// NewCode mints a room code. It reads from crypto/rand and cannot fail: the
// standard library's rand.Read panics rather than returning an error since Go
// 1.24, and there is nothing sensible for a caller to do about an operating
// system that cannot produce randomness anyway.
//
// The buffer is refilled rather than read a byte at a time, so the rejection
// costs one extra syscall in the rare case rather than one per character.
func NewCode() string {
	code := make([]byte, 0, CodeLength)
	buf := make([]byte, CodeLength)

	for len(code) < CodeLength {
		rand.Read(buf)
		for _, b := range buf {
			if int(b) >= codeCeiling {
				continue
			}
			code = append(code, codeAlphabet[int(b)%len(codeAlphabet)])
			if len(code) == CodeLength {
				break
			}
		}
	}

	return string(code)
}

// NormalizeCode is what a typed code goes through before anything looks at it.
// A player types what they were told, which arrives with a stray space off a
// copy-paste and in whatever case their keyboard was in; the stored code is
// upper case, so this is the one place that difference is settled.
func NormalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ValidCode reports whether a value is shaped like a code, and it runs before
// any statement -- exactly as share.ValidToken does. A value that cannot name a
// room does not need a query run to find that out, and the rate limit in front
// of the join is there to bound guesses at real codes rather than typos.
//
// It expects a normalised value: a lower-case code is refused here, because
// accepting one would mean two spellings of the same code reaching the
// database, and the caller has NormalizeCode for that.
func ValidCode(s string) bool {
	if len(s) != CodeLength {
		return false
	}

	for _, c := range s {
		if !strings.ContainsRune(codeAlphabet, c) {
			return false
		}
	}

	return true
}
