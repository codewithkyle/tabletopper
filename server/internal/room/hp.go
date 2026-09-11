package room

import "strings"











const HPEntryLimit = 24




















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





func sumTerms(text string) (int, bool) {
	if len(text) > HPEntryLimit {
		return 0, false
	}

	total, sign, term, digits := 0, 1, 0, 0

	for i := 0; i < len(text); i++ {
		c := text[i]

		if c == '+' || c == '-' {
			
			
			
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
