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

// closeCode is why a connection is being dropped, in the three values this
// design uses. It is a type of our own rather than the library's so that the
// actor -- which decides every one of these -- does not import a WebSocket
// package to say "this person was removed from the room".
type closeCode int

const (
	// closeNormal is an ordinary end. The client reconnects unless it was told
	// the room closed.
	closeNormal closeCode = iota

	// closeGoingAway is a restart. The client reconnects, and its backoff is
	// what spreads a table's worth of browsers out across the new process.
	closeGoingAway

	// closePolicy is a refusal: too slow, over the rate limit, or removed from
	// the room. Only the kick stops the client from reconnecting, and it is
	// the reason string rather than the code that says so.
	closePolicy
)

// The close reasons. They are constants because the client compares one of them
// exactly -- a kicked browser must not reconnect -- and a reason that drifted
// between the two sides would put somebody back in a room they were removed
// from.
const (
	reasonKicked     = "kicked"
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

// client is one connection, from the room goroutine's point of view: somewhere
// to put frames and a way to end it. There is deliberately no socket in here.
//
// THAT IS WHAT MAKES THE ACTOR TESTABLE. A room's tests build clients, read
// their channels and assert on the frames, with no listener, no dial and no
// timing -- the socket half is the two pumps below, and they have exactly one
// job each.
type client struct {
	who    room.Actor
	player room.Player

	// out is the send buffer, and its capacity is the backpressure policy. See
	// actor.send for what happens when it fills.
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

// enqueue puts one frame in the send buffer without ever blocking, and reports
// false when the buffer is full -- which is the only signal the room has that a
// client is not keeping up.
func (c *client) enqueue(frame []byte) bool {
	select {
	case <-c.quit:
		// Already on its way out. Reporting success keeps the room from
		// treating a closing connection as a slow one and logging it as such.
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

// stop ends the connection with a code and a reason, once. The write pump sends
// whatever is still queued before the close frame, so a kicked player receives
// the explanation and then the close rather than the other way round.
func (c *client) stop(code closeCode, reason string) {
	c.once.Do(func() {
		c.code, c.reason = code, reason
		close(c.quit)
	})
}

// attach runs one socket for as long as it lives. It is the only function in
// the package that touches both a WebSocket and a room.
func (h *Hub) attach(w http.ResponseWriter, r *http.Request, a *actor, p room.Player) {
	// No AcceptOptions, and the omission is the policy: the library's default
	// origin check requires the Origin header's host to equal the request's
	// host, which is exactly what this app wants -- the socket is same-origin
	// and rides the session cookie. Naming OriginPatterns here would only ever
	// loosen it. Compression stays off: deflate on hundred-byte frames costs
	// CPU for nothing, and the snapshot is the only frame big enough to gain.
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Warn("Failed to upgrade a room socket", "room", a.id, "error", err)

		return
	}
	defer func() { _ = ws.CloseNow() }()

	ws.SetReadLimit(h.opts.ReadLimit)

	// The socket outlives the request, so it gets a context of its own. The
	// request's is cancelled the moment this handler returns, which for a
	// hijacked connection is a promise about the wrong lifetime.
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

	h.readPump(ctx, ws, a, c)

	// The read loop ending is the connection ending, whichever side caused it.
	// stop is idempotent, so a kick or a slow-client drop that already set a
	// code keeps it.
	c.stop(closeNormal, "")
	<-written

	leaveCtx, leaveCancel := context.WithTimeout(context.WithoutCancel(r.Context()), storeTimeout)
	defer leaveCancel()
	_ = a.post(leaveCtx, leave{c: c})
}

// writePump is the only writer on the socket, which is what makes the ordering
// guarantee real: frames leave in the order the room queued them because one
// goroutine puts them on the wire.
func writePump(ctx context.Context, ws *websocket.Conn, c *client) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-c.quit:
			// Flush before closing. The kick is the case this is for: the
			// player is told why and then the socket ends, and a close that
			// raced the explanation would leave them with neither.
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

// readPump decodes frames and posts them to the room. It runs on the HTTP
// handler's goroutine, so the handler stays inside ServeHTTP for the life of
// the connection and net/http does not tidy up underneath it.
func (h *Hub) readPump(ctx context.Context, ws *websocket.Conn, a *actor, c *client) {
	limit := newBucket(h.opts)

	for {
		kind, data, err := ws.Read(ctx)
		if err != nil {
			return
		}

		switch allowed, over := limit.take(time.Now()); {
		case over:
			// Repeatedly over inside the window. This is not a browser doing
			// what a browser does, so it is ended rather than answered.
			c.stop(closePolicy, reasonRateLimit)

			return
		case !allowed:
			h.post(ctx, a, command{c: c, err: rateLimited()})

			continue
		}

		if kind != websocket.MessageText {
			// The binary door is deliberately left open in the protocol -- a
			// stroke chunk could become int16 pairs one day -- but nothing
			// speaks it yet, so a binary frame is a client this server does
			// not know rather than a feature.
			h.post(ctx, a, command{c: c, err: notText()})

			continue
		}

		cmd, cid, err := room.DecodeCommand(data)
		if err != nil {
			h.post(ctx, a, command{c: c, cid: cid, err: err})

			continue
		}

		// RESOLUTION HAPPENS HERE, ON THIS GOROUTINE, and that is the reason
		// it is not in the room: the three commands that need rows would
		// otherwise have a table full of people waiting behind a SELECT.
		if err := h.resolve(ctx, cmd); err != nil {
			h.post(ctx, a, command{c: c, cid: cid, err: err})

			continue
		}

		h.post(ctx, a, command{c: c, cmd: cmd, cid: cid})
	}
}

// post sends one message to the room and gives up if the room has gone. There
// is nothing to do about that here: the room unloaded, so this socket is about
// to be closed by the same event.
func (h *Hub) post(ctx context.Context, a *actor, m any) {
	_ = a.post(ctx, m)
}

// bucket is the per-connection command rate limit: a token bucket at Rate a
// second with room for Burst, and a count of how often it has been emptied.
//
// THE COUNT IS WHAT ENDS THE CONNECTION. One over is a client that ran a macro
// or held a key down, and the answer to that is an error frame it can show.
// Several inside a minute is not a browser, and the answer to that is the door.
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

// take reports whether this frame may be processed, and whether the connection
// has been over the limit often enough to end.
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
