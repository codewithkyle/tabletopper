package room

import "strings"

// THE HIT-POINT BOX'S ARITHMETIC, which is a rule about a pawn's hit points
// and lives beside the clamp that finishes it. The route that reads the box
// calls it; the client resolves the same strings so the number appears without
// a round trip; and the fixture in testdata/rules/hp.json is what holds the two
// together.

// HPEntryLimit is how long a hit-point entry may be. It is not a rule about hit
// points -- the core decides those -- but a bound on the arithmetic, so a
// pasted essay is refused as an entry rather than summed a digit at a time. It
// is js/room/hp.ts's LIMIT and the maxlength the box carries.
const HPEntryLimit = 24

// EvaluateHP is the arithmetic a hit-point box takes: 12 sets, -7 subtracts,
// and 23-7-4 is worked out.
//
// THE SIGN IS THE OPERATOR AND THE ABSENCE OF ONE IS ALSO A DECISION. "7" in a
// box showing 12 means seven, not nineteen; a GM setting a monster's hit points
// to a number reads it off a sheet, and a GM applying damage types the minus
// sign they would say out loud. Clamping is left to the core, which does it
// against the max hit points it holds rather than the ones this happens to have
// been handed.
//
// THE CLIENT EVALUATES THE SAME STRINGS AND THIS IS THE AUTHORITY. js/room/hp.ts
// resolves the box on blur so the number appears without a round trip; every
// case it answers, this has to answer the same way, and hp.test.ts and
// the hp fixture in testdata/rules are the two halves of that agreement. A request
// still arrives carrying a sum whenever that script has not run.
//
// AN EMPTY BOX IS NOT A CHANGE, which is the false in the middle answer. A pawn
// may have no hit points recorded at all, and somebody clearing a box to retype
// it must not blur their way into setting the goblin to zero.
func EvaluateHP(entry string, current *int) (int, bool, string) {
	text := strings.Join(strings.Fields(entry), " ")
	text = strings.ReplaceAll(text, " +", "+")
	text = strings.ReplaceAll(text, "+ ", "+")
	text = strings.ReplaceAll(text, " -", "-")
	text = strings.ReplaceAll(text, "- ", "-")

	if text == "" {
		return 0, false, ""
	}

	total, ok := sumTerms(text)
	if !ok {
		return 0, false, "has to be a number, or a change such as -7 or 23-7."
	}

	if text[0] != '+' && text[0] != '-' {
		return total, true, ""
	}

	from := 0
	if current != nil {
		from = *current
	}

	return from + total, true, ""
}

// sumTerms adds a chain of signed whole numbers, left to right, and answers
// false for anything that is not one. There is no precedence to get wrong: the
// only operators are plus and minus, which is the whole of what a table does to
// a hit-point total.
func sumTerms(text string) (int, bool) {
	if len(text) > HPEntryLimit {
		return 0, false
	}

	total, sign, term, digits := 0, 1, 0, 0

	for i := 0; i < len(text); i++ {
		c := text[i]

		if c == '+' || c == '-' {
			// A sign is only an operator after a number, and only ever the
			// first character otherwise -- so "23--7" and a bare "-" are
			// refused rather than read as something nobody typed.
			if i > 0 && digits == 0 {
				return 0, false
			}

			total += sign * term
			sign, term, digits = 1, 0, 0
			if c == '-' {
				sign = -1
			}

			continue
		}

		if c < '0' || c > '9' {
			return 0, false
		}

		term = term*10 + int(c-'0')
		digits++
	}

	if digits == 0 {
		return 0, false
	}

	return total + sign*term, true
}
