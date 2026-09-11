package share

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"runtime"

	"golang.org/x/crypto/bcrypt"
)

const (
	tokenBytes   = 16
	PasswordMin  = 6
	PasswordMax  = 72
	unlockCookie = "share_unlock"
	unlockWindow = 12 * 60 * 60
)

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
func NewToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("share: token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("share: hash password: %w", err)
	}
	return string(hash), nil
}

var bcryptSlots = make(chan struct{}, runtime.GOMAXPROCS(0))

func PasswordMatches(ctx context.Context, hash, plain string) (bool, error) {
	select {
	case bcryptSlots <- struct{}{}:
		defer func() { <-bcryptSlots }()
	case <-ctx.Done():
		return false, ctx.Err()
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil, nil
}
func Unlocked(r *http.Request, token, hash string) bool {
	cookie, err := r.Cookie(unlockCookie)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(cookie.Value), []byte(unlockProof(token, hash)))
}
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
