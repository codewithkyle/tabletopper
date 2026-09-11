package hub
import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
	"tabletopper/internal/room"
	"github.com/coder/websocket"
)
const (
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
	membershipInterval = 2 * time.Minute
)
type Membership func(ctx context.Context) bool
type closeCode int
const (
	closeNormal closeCode = iota
	closeGoingAway
	closePolicy
)
const (
	reasonKicked     = "kicked"
	reasonLeft       = "left"
	reasonLimit      = "limit"
	reasonSlow       = "slow"
	reasonClosed     = "closed"
	reasonRestarting = "restarting"
	reasonRateLimit  = "rate"
)
func (c closeCode) status() websocket.StatusCode {
	switch c {
	case closeGoingAway:
		return websocket.StatusGoingAway
	case closePolicy:
		return websocket.StatusPolicyViolation
	default:
		return websocket.StatusNormalClosure
	}
}
type client struct {
	who    room.Actor
	player room.Player
	out chan []byte
	once   sync.Once
	quit   chan struct{}
	code   closeCode
	reason string
}
func newClient(p room.Player, buffer int) *client {
	return &client{
		who:    room.Actor{ID: p.ID, Role: p.Role},
		player: p,
		out:    make(chan []byte, buffer),
		quit:   make(chan struct{}),
	}
}
func (c *client) enqueue(frame []byte) bool {
	select {
	case <-c.quit:
		return true
	default:
	}
	select {
	case c.out <- frame:
		return true
	default:
		return false
	}
}
func (c *client) stop(code closeCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.quit)
	})
}
func (h *Hub) attach(w http.ResponseWriter, r *http.Request, a *actor, p room.Player, member Membership) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Warn("Failed to upgrade a room socket", "room", a.id, "error", err)
		return
	}
	defer func() { _ = ws.CloseNow() }()
	ws.SetReadLimit(h.opts.ReadLimit)
	ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer cancel()
	c := newClient(p, h.opts.SendBuffer)
	if err := a.post(ctx, join{c: c}); err != nil {
		_ = ws.Close(websocket.StatusGoingAway, reasonRestarting)
		return
	}
	written := make(chan struct{})
	go func() {
		defer close(written)
		writePump(ctx, ws, c)
	}()
	go watch(ctx, ws, member, c, cancel)
	h.readPump(ctx, ws, a, c)
	c.stop(closeNormal, "")
	<-written
	leaveCtx, leaveCancel := context.WithTimeout(context.WithoutCancel(r.Context()), storeTimeout)
	defer leaveCancel()
	_ = a.post(leaveCtx, leave{c: c})
}
type pinger interface {
	Ping(ctx context.Context) error
}
func watch(ctx context.Context, ws pinger, member Membership, c *client, end func()) {
	watchOn(ctx, ws, member, c, end, pingInterval, membershipInterval)
}
func watchOn(ctx context.Context, ws pinger, member Membership, c *client, end func(), ping, recheck time.Duration) {
	pings := time.NewTicker(ping)
	defer pings.Stop()
	var rechecks <-chan time.Time
	if member != nil {
		t := time.NewTicker(recheck)
		defer t.Stop()
		rechecks = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-pings.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := ws.Ping(pingCtx)
			cancel()
			if err != nil {
				end()
				return
			}
		case <-rechecks:
			checkCtx, cancel := context.WithTimeout(ctx, storeTimeout)
			ok := member(checkCtx)
			cancel()
			if !ok {
				c.stop(closePolicy, "")
				end()
				return
			}
		}
	}
}
func writePump(ctx context.Context, ws *websocket.Conn, c *client) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.quit:
			for {
				select {
				case frame := <-c.out:
					if !write(ctx, ws, frame) {
						return
					}
					continue
				default:
				}
				break
			}
			_ = ws.Close(c.code.status(), c.reason)
			return
		case frame := <-c.out:
			if !write(ctx, ws, frame) {
				return
			}
		}
	}
}
func write(ctx context.Context, ws *websocket.Conn, frame []byte) bool {
	ctx, cancel := context.WithTimeout(ctx, writeDeadline)
	defer cancel()
	return ws.Write(ctx, websocket.MessageText, frame) == nil
}
func (h *Hub) readPump(ctx context.Context, ws *websocket.Conn, a *actor, c *client) {
	limit := newBucket(h.opts)
	for {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		switch allowed, over := limit.take(time.Now()); {
		case over:
			c.stop(closePolicy, reasonRateLimit)
			return
		case !allowed:
			h.post(ctx, a, command{c: c, err: rateLimited()})
			continue
		}
		if kind != websocket.MessageText {
			h.post(ctx, a, command{c: c, err: notText()})
			continue
		}
		cmd, cid, err := room.DecodeCommand(data)
		if err != nil {
			h.post(ctx, a, command{c: c, cid: cid, err: err})
			continue
		}
		if err := h.resolve(ctx, a.id, c.who, cmd); err != nil {
			h.post(ctx, a, command{c: c, cid: cid, err: err})
			continue
		}
		h.post(ctx, a, command{c: c, cmd: cmd, cid: cid})
	}
}
func (h *Hub) post(ctx context.Context, a *actor, m any) {
	_ = a.post(ctx, m)
}
type bucket struct {
	rate  float64
	burst float64
	opts  Options
	tokens float64
	last   time.Time
	overs     int
	overSince time.Time
}
func newBucket(o Options) *bucket {
	return &bucket{
		rate:   float64(o.Rate),
		burst:  float64(o.Burst),
		opts:   o,
		tokens: float64(o.Burst),
		last:   time.Now(),
	}
}
func (b *bucket) take(now time.Time) (allowed, over bool) {
	if elapsed := now.Sub(b.last); elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed.Seconds()*b.rate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, false
	}
	if b.overSince.IsZero() || now.Sub(b.overSince) > b.opts.OverWindow {
		b.overSince, b.overs = now, 0
	}
	b.overs++
	return false, b.overs >= b.opts.Overs
}
func rateLimited() error {
	return &room.Error{
		Code:    room.CodeRateLimited,
		Heading: "Slow down",
		Message: "That was more than the server will take at once. Try again in a moment.",
	}
}
func notText() error {
	return &room.Error{
		Code:    room.CodeInvalid,
		Heading: "Bad message",
		Message: "The server only reads text frames on this connection.",
	}
}
