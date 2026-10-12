package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHSTSIsSentOnlyOverHTTPS(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for name, c := range map[string]struct {
		tls       bool
		forwarded string
		want      string
	}{
		"plain http":                          {want: ""},
		"a proxy that terminated TLS":         {forwarded: "https", want: strictTransport},
		"a proxy that forwarded http":         {forwarded: "http", want: ""},
		"TLS straight to the server":          {tls: true, want: strictTransport},
		"a forwarded scheme it does not know": {forwarded: "gopher", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", c.forwarded)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if got := rec.Header().Get("Strict-Transport-Security"); got != c.want {
				t.Errorf("Strict-Transport-Security = %q, want %q", got, c.want)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff on every response", got)
			}
		})
	}
}
