




package room

import (
	"crypto/rand"
	"strconv"
	"strings"
)







const CodeLength = 4









const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"




const codeCeiling = 256 - 256%len(codeAlphabet)












func CodePattern() string {
	return "[" + codeAlphabet + strings.ToLower(codeAlphabet) + "]{" + strconv.Itoa(CodeLength) + "}"
}








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





func NormalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}









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
