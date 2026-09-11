package share_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tabletopper/internal/share"
)

func unlock(t *testing.T, setToken, setHash, readToken, readHash string) bool {
	t.Helper()
	w := httptest.NewRecorder()
	share.SetUnlocked(w, setToken, setHash, true)
	r := httptest.NewRequest(http.MethodGet, "/share/"+readToken, nil)
	for _, cookie := range w.Result().Cookies() {
		r.AddCookie(cookie)
	}
	return share.Unlocked(r, readToken, readHash)
}
func matches(t *testing.T, hash, plain string) bool {
	t.Helper()
	ok, err := share.PasswordMatches(context.Background(), hash, plain)
	if err != nil {
		t.Fatalf("PasswordMatches: %v", err)
	}
	return ok
}
func hash(t *testing.T, plain string) string {
	t.Helper()
	h, err := share.HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return h
}
func TestTokensAreDistinctAndURLSafe(t *testing.T) {
	seen := make(map[string]bool, 64)
	for range 64 {
		token, err := share.NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if seen[token] {
			t.Fatalf("NewToken repeated %q", token)
		}
		seen[token] = true
		if strings.ContainsAny(token, "+/=") {
			t.Errorf("a token has to survive being pasted into a URL: %q", token)
		}
	}
}
func TestAPasswordVerifiesAgainstItsOwnHashAndNoOther(t *testing.T) {
	h := hash(t, "the black spider")
	if !matches(t, h, "the black spider") {
		t.Error("the right password did not match its hash")
	}
	if matches(t, h, "the black spiders") {
		t.Error("a wrong password matched")
	}
}
func TestABusyProcessRefusesTheCheckRatherThanFailingIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ok, err := share.PasswordMatches(ctx, hash(t, "phandalin"), "phandalin")
	if err == nil {
		t.Fatal("a cancelled check reported no error")
	}
	if ok {
		t.Error("a cancelled check reported a match")
	}
}
func TestAnUnlockedBrowserComesBackUnlocked(t *testing.T) {
	h := hash(t, "phandalin")
	if !unlock(t, "tokenA", h, "tokenA", h) {
		t.Error("the cookie this share issued did not verify against it")
	}
}
func TestAGrantForOneShareDoesNotOpenAnother(t *testing.T) {
	h := hash(t, "phandalin")
	if unlock(t, "tokenA", h, "tokenB", h) {
		t.Error("a cookie issued for one share verified against another")
	}
}
func TestChangingThePasswordInvalidatesAnOutstandingGrant(t *testing.T) {
	if unlock(t, "tokenA", hash(t, "phandalin"), "tokenA", hash(t, "phandalin")) {
		t.Error("a cookie verified against a re-hash of the same password")
	}
}
func TestNoCookieIsNotUnlocked(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/share/tokenA", nil)
	if share.Unlocked(r, "tokenA", hash(t, "phandalin")) {
		t.Error("a request with no cookie was treated as unlocked")
	}
}
func TestTheGrantIsScopedToItsOwnShare(t *testing.T) {
	w := httptest.NewRecorder()
	share.SetUnlocked(w, "tokenA", hash(t, "phandalin"), true)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	if got, want := cookies[0].Path, "/share/tokenA"; got != want {
		t.Errorf("cookie path is %q, want %q", got, want)
	}
	if !cookies[0].HttpOnly {
		t.Error("the grant should not be readable from a script")
	}
}
func TestAPasswordAtBcryptsCeilingIsStillHashable(t *testing.T) {
	if _, err := share.HashPassword(strings.Repeat("a", share.PasswordMax)); err != nil {
		t.Errorf("a password of exactly PasswordMax bytes should hash: %v", err)
	}
	if _, err := share.HashPassword(strings.Repeat("a", share.PasswordMax+1)); err == nil {
		t.Error("PasswordMax is meant to be the last length bcrypt accepts")
	}
}
