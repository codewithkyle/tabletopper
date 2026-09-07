package share

import (
	"sync"
	"time"
)

// Attempts counts password tries per share, and it is the thing that makes a
// share password worth having. bcrypt's cost bounds what one guess costs; it
// does not bound how many guesses arrive, and without a counter in front of it
// the default cost is a throughput figure rather than a defence -- roughly
// fifteen tries a second per core against a six-character floor, which
// finishes.
//
// THE KEY IS THE TOKEN AND NOT THE CLIENT. Every request that reaches here
// arrived through Cloudflare, so the only address available is one the edge
// wrote into a header, and a counter keyed by something the client can
// influence is a counter that resets when the attacker asks it to. Keying by
// the share means a flood against one link locks that one link for the window
// and leaves every other share untouched. The reader it locks out is the reader
// somebody is already attacking, and a minute is the whole cost to them.
//
// It is in-process state, which is enough for the one instance this runs as. A
// second instance would need the count on the shares row, and would want the
// window there too so the two agreed on when it started.
type Attempts struct {
	limit  int
	window time.Duration

	mu   sync.Mutex
	seen map[string]attempt
}

// attempt is one share's window: how many tries have landed in it, and when it
// opened. A window is not slid -- it is replaced once it has run out, so the
// count is of tries since `since` rather than of the last `limit` tries, and
// nothing has to be kept per try to work that out.
type attempt struct {
	count int
	since time.Time
}

// NewAttempts builds a counter allowing limit tries per share per window.
func NewAttempts(limit int, window time.Duration) *Attempts {
	return &Attempts{limit: limit, window: window, seen: map[string]attempt{}}
}

// Allow records one try against token and reports whether it may proceed. Ask
// once per attempt, and ask before the password is checked -- the point is to
// keep bcrypt from running, so a call made afterwards has already paid for the
// thing it was meant to prevent.
//
// The refusal outlasts the limit on purpose: a try that is turned away still
// counts, so a caller hammering a locked share holds it locked rather than
// walking the count down.
func (a *Attempts) Allow(token string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Sweeping here rather than on a timer, because the map only needs
	// collecting when it has grown, and the only thing that grows it is this
	// call. The bound is on distinct password-protected shares under attack at
	// once -- the caller has already found a real row by the time it asks --
	// so 4096 is a ceiling this app is not expected to reach and not a limit
	// anything is meant to run against.
	if len(a.seen) > 4096 {
		for k, v := range a.seen {
			if now.Sub(v.since) >= a.window {
				delete(a.seen, k)
			}
		}
	}

	at := a.seen[token]
	if now.Sub(at.since) >= a.window {
		at = attempt{since: now}
	}
	at.count++
	a.seen[token] = at

	return at.count <= a.limit
}
