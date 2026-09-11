package share
import (
	"sync"
	"time"
)
type Attempts struct {
	limit  int
	window time.Duration
	mu   sync.Mutex
	seen map[string]attempt
}
type attempt struct {
	count int
	since time.Time
}
func NewAttempts(limit int, window time.Duration) *Attempts {
	return &Attempts{limit: limit, window: window, seen: map[string]attempt{}}
}
func (a *Attempts) Allow(token string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
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
