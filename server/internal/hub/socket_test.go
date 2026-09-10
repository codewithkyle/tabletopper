package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tabletopper/internal/room"

	"github.com/coder/websocket"
)

// THE ONE TEST WITH A SOCKET IN IT. Everything else in this package drives the
// room through its inbox, because that is where the behaviour is; this is here
// to prove that the two halves are joined -- that an upgrade succeeds, that the
// first frame down the wire is the snapshot, and that a command sent by one
// browser comes back to both.
func TestTwoBrowsersInOneRoomSeeTheSameEvent(t *testing.T) {
	tb := newTabletop(t, Options{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := room.Player{ID: playerID, Name: "Ari", Role: room.RolePlayer}
		if r.URL.Query().Get("as") == "gm" {
			p = room.Player{ID: gmID, Name: "Kyle", Role: room.RoleGM}
		}

		tb.Serve(w, r, roomID, p, nil)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gm := dial(t, ctx, srv.URL+"?as=gm")
	defer func() { _ = gm.CloseNow() }()
	player := dial(t, ctx, srv.URL)
	defer func() { _ = player.CloseNow() }()

	// The first frame either browser sees is its own room, whole.
	gmSnapshot := next(t, ctx, gm)
	if gmSnapshot.Type != "snapshot" {
		t.Fatalf("the GM's first frame was %q, want the snapshot", gmSnapshot.Type)
	}
	playerSnapshot := next(t, ctx, player)
	if playerSnapshot.Type != "snapshot" {
		t.Fatalf("the player's first frame was %q, want the snapshot", playerSnapshot.Type)
	}

	// The GM was already connected when the player arrived, so the GM is told.
	joined := next(t, ctx, gm)
	if joined.Type != "player.joined" {
		t.Fatalf("the GM's second frame was %q, want player.joined", joined.Type)
	}

	layer := activeLayer(t, playerSnapshot)
	send(t, ctx, player, map[string]any{"type": "ping", "cid": "p1", "layer": layer.String(), "x": 320, "y": 240})

	// Everybody including the pinger, which is the point of the feature: one
	// marker in one place on every screen at the same time.
	for who, c := range map[string]*websocket.Conn{"the GM": gm, "the pinger": player} {
		got := next(t, ctx, c)
		if got.Type != "pinged" {
			t.Errorf("%s received %q, want pinged", who, got.Type)
		}
		if x, _ := got.Body["x"].(float64); x != 320 {
			t.Errorf("%s received the ping at x=%v, want 320", who, x)
		}
		if by, _ := got.Body["by"].(string); by != playerID.String() {
			t.Errorf("%s was told %q pinged, want the player", who, by)
		}
	}

	// A frame no generated client could produce is refused to the sender
	// alone, with the correlation id it was sent with.
	send(t, ctx, player, map[string]any{"type": "table.setGrid", "cid": "p2", "nonsense": true})
	refusal := next(t, ctx, player)
	if refusal.Type != "error" || refusal.CID != "p2" {
		t.Errorf("the refusal was %q with cid %q, want an error for p2", refusal.Type, refusal.CID)
	}
}

func dial(t *testing.T, ctx context.Context, url string) *websocket.Conn {
	t.Helper()

	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(url, "http"), nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}

	return c
}

func next(t *testing.T, ctx context.Context, c *websocket.Conn) frame {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	kind, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if kind != websocket.MessageText {
		t.Fatalf("frame type = %v, want text", kind)
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("a frame was not JSON: %v\n%s", err, data)
	}

	f := frame{Body: body}
	f.Type, _ = body["type"].(string)
	f.CID, _ = body["cid"].(string)
	if seq, ok := body["seq"].(float64); ok {
		f.Seq = uint64(seq)
	}

	return f
}

func send(t *testing.T, ctx context.Context, c *websocket.Conn, cmd map[string]any) {
	t.Helper()

	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}
