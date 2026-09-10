package hub

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/room"
)

// THE KICK RACE. A kick clears the person's session rows, and a tab of theirs
// that was in reconnect backoff at that instant never saw the close reason: it
// comes back, and if its upgrade read the rows before the clear committed, the
// membership check passes and the join would seat them again. The room itself
// remembers the kick, so that join is refused whatever the rows said.
func TestAJoinFromSomebodyJustKickedIsRefusedByTheRoom(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)

	tb.send(gm, "k", &room.PlayerKick{ID: playerID})
	only(t, player, "player.kicked")
	only(t, gm, "player.left")

	// The reconnect, arriving as a fresh connection for the same person.
	again := tb.join(playerID, "Ari", room.RolePlayer)

	select {
	case <-again.quit:
	default:
		t.Fatal("the kicked player's reconnect was seated")
	}
	if again.reason != reasonKicked {
		t.Errorf("close reason = %q, want %q so the client stops trying", again.reason, reasonKicked)
	}
	if rest := frames(t, again); len(rest) != 0 {
		t.Errorf("the refused connection received %v, want nothing", types(rest))
	}

	// Nobody else heard an arrival, and the player list still does not name
	// them.
	if rest := frames(t, gm); len(rest) != 0 {
		t.Errorf("the GM received %v after a refused join, want nothing", types(rest))
	}
	players, _ := tb.Players(tb.ctx(), roomID)
	for _, p := range players {
		if p.ID == playerID {
			t.Error("the kicked player is back in the player list")
		}
	}
}

// A person who pressed Leave in one tab meant it in all of them. The row is
// gone from the state, so a tab still connected would be a socket whose every
// command is checked against an id the player list no longer names.
func TestLeavingInOneTabClosesTheOthers(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	first := tb.join(playerID, "Ari", room.RolePlayer)
	second := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, first)
	frames(t, second)

	// The Leave button is an HTTP route, which mirrors into the room.
	tb.Notify(roomID, &room.PlayerLeave{ID: playerID})
	tb.settle()

	for name, c := range map[string]*client{"the first tab": first, "the second tab": second} {
		select {
		case <-c.quit:
		default:
			t.Errorf("%s was left open after the person left", name)
		}
		if c.reason != reasonLeft {
			t.Errorf("%s: close reason = %q, want %q", name, c.reason, reasonLeft)
		}
	}

	only(t, gm, "player.left")
	select {
	case <-gm.quit:
		t.Error("the GM's connection was closed by somebody else leaving")
	default:
	}
}

// The caps. A join costs the room an apply, a broadcast and a whole projected
// snapshot, and the rate limit is per socket, so a person with a script is N
// sockets multiplying both. Past the cap the connection is refused before it
// is seated, and the ones already open are unaffected.
func TestConnectionsPastTheCapAreRefusedAndTheRestUnaffected(t *testing.T) {
	t.Run("per person", func(t *testing.T) {
		tb := newTabletop(t, Options{ConnsPerUser: 2})

		gm := tb.join(gmID, "Kyle", room.RoleGM)
		first := tb.join(playerID, "Ari", room.RolePlayer)
		second := tb.join(playerID, "Ari", room.RolePlayer)
		third := tb.join(playerID, "Ari", room.RolePlayer)

		refused(t, third, reasonLimit)
		for name, c := range map[string]*client{"the GM": gm, "the first tab": first, "the second tab": second} {
			select {
			case <-c.quit:
				t.Errorf("%s was closed by somebody else's refused connection", name)
			default:
			}
		}

		// And somebody else is still welcome: the cap is per person.
		other := tb.join(otherID, "Rin", room.RolePlayer)
		only(t, other, "snapshot")
	})

	t.Run("per room", func(t *testing.T) {
		tb := newTabletop(t, Options{ConnsPerRoom: 2})

		tb.join(gmID, "Kyle", room.RoleGM)
		tb.join(playerID, "Ari", room.RolePlayer)
		third := tb.join(otherID, "Rin", room.RolePlayer)

		refused(t, third, reasonLimit)
	})
}

func refused(t *testing.T, c *client, reason string) {
	t.Helper()

	select {
	case <-c.quit:
	default:
		t.Fatal("the connection past the cap was seated")
	}
	if c.reason != reason {
		t.Errorf("close reason = %q, want %q", c.reason, reason)
	}
	if got := frames(t, c); len(got) != 0 {
		t.Errorf("the refused connection received %v, want nothing", types(got))
	}
}

// fakePinger answers pings until it is told to fail.
type fakePinger struct {
	fail chan struct{}
}

func (p *fakePinger) Ping(ctx context.Context) error {
	select {
	case <-p.fail:
		return errors.New("no pong")
	default:
		return nil
	}
}

// THE KEEPALIVE. A peer that vanished without a FIN -- a laptop lid, a NAT
// table, a mobile radio -- stays connected until something notices, and in a
// quiet room nothing writes to it. The watchdog pings, and a ping that is not
// answered ends the socket, which is what posts the leave.
func TestTheWatchdogEndsASocketThatStopsAnsweringPings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pings := &fakePinger{fail: make(chan struct{})}
	c := newClient(room.Player{ID: playerID, Role: room.RolePlayer}, 4)
	ended := make(chan struct{})

	go watchOn(ctx, pings, nil, c, func() { close(ended) }, 5*time.Millisecond, time.Hour)

	// Answered pings keep it open.
	time.Sleep(30 * time.Millisecond)
	select {
	case <-ended:
		t.Fatal("the watchdog ended a socket whose pings were answered")
	default:
	}

	close(pings.fail)
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("the watchdog did not end a socket whose ping failed")
	}
}

// THE RE-CHECK. Membership was checked once, at the upgrade, and a logout, a
// leave from another tab or a deleted room all change the answer without
// touching the socket. The watchdog asks again on a timer and closes a socket
// whose answer has become no.
func TestTheWatchdogClosesASocketWhoseMembershipLapsed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	member := make(chan bool, 1)
	member <- true
	check := func(context.Context) bool {
		select {
		case ok := <-member:
			return ok
		default:
			return false
		}
	}

	c := newClient(room.Player{ID: playerID, Role: room.RolePlayer}, 4)
	ended := make(chan struct{})

	go watchOn(ctx, &fakePinger{fail: make(chan struct{})}, check, c, func() { close(ended) }, time.Hour, 5*time.Millisecond)

	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("the watchdog did not end a socket whose membership lapsed")
	}

	select {
	case <-c.quit:
	default:
		t.Fatal("the socket's client was not stopped")
	}
	if c.code != closePolicy {
		t.Errorf("close code = %v, want a policy violation", c.code)
	}
}

// THE CLOSE REASONS THE CLIENT ENDS ON are this package's words, and the client
// compares them by exact string. socket.ts lists them in ENDED_REASONS; this
// reads that file so the two lists cannot drift -- a reason spelled differently
// on one side is a person the GM removed whose browser comes straight back.
func TestTheClientEndsOnExactlyTheReasonsTheServerEndsWith(t *testing.T) {
	const source = "../../js/room/socket.ts"

	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("reading %s: %v", source, err)
	}

	start := strings.Index(string(body), "export const ENDED_REASONS")
	if start < 0 {
		t.Fatalf("%s no longer declares ENDED_REASONS", source)
	}
	declaration := string(body)[start:]
	declaration = declaration[:strings.Index(declaration, ";")]

	for _, reason := range []string{reasonKicked, reasonLeft, reasonLimit} {
		if !strings.Contains(declaration, `"`+reason+`"`) {
			t.Errorf("the client does not end on %q: %s", reason, declaration)
		}
	}
	for _, reason := range []string{reasonSlow, reasonClosed, reasonRestarting, reasonRateLimit} {
		if strings.Contains(declaration, `"`+reason+`"`) {
			t.Errorf("the client ends on %q, which it should reconnect from", reason)
		}
	}
}
