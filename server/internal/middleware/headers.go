package middleware

import "net/http"

// SecurityHeaders is the floor every response in the app gets, wrapped around
// the whole mux in main rather than repeated per handler.
//
// A HANDLER THAT WANTS A DIFFERENT VALUE SETS IT AND WINS, because Set replaces
// rather than appends and the handler runs after this. That is not a loophole,
// it is how the two tighter policies in the app stay tighter: shareHeaders
// narrows Referrer-Policy to no-referrer for a page whose URL is the
// credential, and the journal entry page and the shared pages each set their
// own Content-Security-Policy.
//
// X-Frame-Options RATHER THAN A CSP frame-ancestors, and the reason is the
// sentence above. Two handlers already send a Content-Security-Policy of their
// own; adding frame-ancestors to a header set here would be replaced wholesale
// by either of them, so the anti-framing rule would quietly not apply to the
// two pages that render a reader's Markdown. A separate header cannot be
// clobbered by a policy that does not mention it. It is the older mechanism and
// every browser still honours it for DENY, which is the only value used here.
//
// Nothing in this app is ever framed -- there is no embed, no widget and no
// OAuth popup -- and the pages behind a session carry one-click destructive
// buttons behind hx-confirm, which is exactly what a framed page is for.
//
// nosniff, because the image routes label every body image/webp whatever the
// bytes are and the Markdown export is served with a filename rather than as an
// attachment; a browser that goes looking for a better content type than the
// one it was given is a browser deciding one of those is a document.
//
// HSTS IS NOT HERE. It belongs to whatever terminates TLS, which is Cloudflare,
// and a max-age sent by the origin as well would be a second place to get the
// preload decision wrong. If this ever serves TLS itself, it needs adding here.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}
