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






func TestAJoinFromSomebodyJustKickedIsRefusedByTheRoom(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)

	tb.send(gm, "k", &room.PlayerKick{ID: playerID})
	only(t, player, "player.kicked")
	only(t, gm, "player.left")

	
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




func TestLeavingInOneTabClosesTheOthers(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	first := tb.join(playerID, "Ari", room.RolePlayer)
	second := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, first)
	frames(t, second)

	
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





func TestTheWatchdogEndsASocketThatStopsAnsweringPings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pings := &fakePinger{fail: make(chan struct{})}
	c := newClient(room.Player{ID: playerID, Role: room.RolePlayer}, 4)
	ended := make(chan struct{})

	go watchOn(ctx, pings, nil, c, func() { close(ended) }, 5*time.Millisecond, time.Hour)

	
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
