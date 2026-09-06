// Package share is the read grant on one thing -- a journal entry or a whole
// character sheet: the token that names it in a link, the optional password that
// gates it, and the cookie that remembers someone got past the password.
//
// NOTHING IN HERE KNOWS WHICH KIND IT IS HOLDING, and that is why a second kind
// of share cost this package nothing. A token is 128 bits either way, a password
// gates a row either way, and the cookie is keyed by the token and the hash --
// so the grant for one share cannot be replayed at another whether or not the
// two open the same sort of page.
//
// A SHARE LINK IS A BEARER CREDENTIAL, which is what separates the token here
// from every other id in this app. A ULID is unguessable enough, but it is
// also a primary key and a creation timestamp, and a value that is all three
// cannot be rotated without destroying the row it identifies. So a share row
// has an id like everything else and a token beside it, and only the token is
// ever in a URL.
package share

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

const (
	// tokenBytes is the randomness in a link. 128 bits, which is the same
	// order as the session token next door, and base64url-encodes to 22
	// characters with no padding -- short enough to paste into a chat message
	// without it wrapping.
	tokenBytes = 16

	// PasswordMin is a floor rather than a policy. This gates something shared
	// with the four people at a table, not an account, and a rule strict enough
	// to be worth enforcing here would only be answered with a password written
	// down beside the link.
	PasswordMin = 6

	// PasswordMax is bcrypt's own limit, named rather than discovered.
	// bcrypt.GenerateFromPassword returns an error past 72 bytes rather than
	// truncating, so without this a long passphrase would reach the writer as
	// a 500 on a field they were entitled to fill.
	PasswordMax = 72

	// unlockCookie remembers that this browser answered the password. It is
	// path-scoped to the one share it belongs to, so the name is fixed and two
	// shares open in two tabs do not overwrite each other's.
	unlockCookie = "share_unlock"

	// unlockWindow is how long an answered password stays answered. Long
	// enough to read an entry and come back to it after dinner, short enough
	// that a borrowed laptop is not still holding the grant next week.
	unlockWindow = 12 * 60 * 60
)

// ValidToken reports whether a path segment is shaped like one of ours. It is
// the cheap refusal in front of every share route: a token is a fixed length
// and a fixed alphabet, so anything else cannot name a row and does not need a
// query run to find that out. It says nothing about whether the share exists.
func ValidToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(tokenBytes) {
		return false
	}

	for _, c := range []byte(token) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}

	return true
}

// NewToken mints the random half of a share link.
func NewToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("share: token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// HashPassword is bcrypt at the default cost, and the cost is the rate limit.
// There is nothing in front of the unlock form counting attempts; a guess costs
// the ~60ms bcrypt takes to check it, which is what makes a share password
// worth having without a counter behind it.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("share: hash password: %w", err)
	}

	return string(hash), nil
}

// PasswordMatches reports whether plain is the password behind hash.
func PasswordMatches(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Unlocked reports whether this request already answered the share's password.
//
// THE PROOF IS DERIVED FROM THE STORED HASH RATHER THAN FROM A SECRET OR A
// SECOND TABLE. The cookie holds HMAC-SHA256 of the token keyed by the bcrypt
// hash, which the browser never sees; recomputing it needs the row, so a
// cookie cannot be forged without the database, and nothing has to be written
// down to check one. Three properties come free from that choice: revoking the
// share deletes the row and the cookie stops verifying, changing the password
// changes the key and every outstanding cookie dies with it, and a grant for
// one share cannot be replayed at another because the token is what is signed.
//
// hmac.Equal rather than ==, so a wrong cookie does not leak where it went
// wrong through how long the comparison took.
func Unlocked(r *http.Request, token, hash string) bool {
	cookie, err := r.Cookie(unlockCookie)
	if err != nil {
		return false
	}

	return hmac.Equal([]byte(cookie.Value), []byte(unlockProof(token, hash)))
}

// SetUnlocked writes the grant this browser just earned.
//
// PATH IS THE SCOPING, and it is why one cookie name serves every share: the
// browser only sends this back to the share it was issued for, so a reader who
// has answered one password is not silently carrying a grant into another
// link. HttpOnly because no script has any use for it, and SameSite=Lax so a
// link followed from a chat window still arrives unlocked.
func SetUnlocked(w http.ResponseWriter, token, hash string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     unlockCookie,
		Value:    unlockProof(token, hash),
		Path:     "/share/" + token,
		MaxAge:   unlockWindow,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func unlockProof(token, hash string) string {
	mac := hmac.New(sha256.New, []byte(hash))
	mac.Write([]byte(token))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
