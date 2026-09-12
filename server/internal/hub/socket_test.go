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
	gmSnapshot := next(t, ctx, gm)
	if gmSnapshot.Type != "snapshot" {
		t.Fatalf("the GM's first frame was %q, want the snapshot", gmSnapshot.Type)
	}
	playerSnapshot := next(t, ctx, player)
	if playerSnapshot.Type != "snapshot" {
		t.Fatalf("the player's first frame was %q, want the snapshot", playerSnapshot.Type)
	}
	joined := next(t, ctx, gm)
	if joined.Type != "changes" {
		t.Fatalf("the GM's second frame was %q, want the changes frame", joined.Type)
	}
	if got := carried(t, joined); !equal(got, []string{"players.upserted"}) {
		t.Fatalf("the frame carries %v, want the arrival alone", got)
	}
	layer := activeLayer(t, playerSnapshot)
	send(t, ctx, player, map[string]any{"type": "ping", "cid": "p1", "layer": layer.String(), "x": 320, "y": 240})
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
func carried(t *testing.T, f frame) []string {
	t.Helper()
	events, ok := f.Body["events"].([]any)
	if !ok {
		t.Fatalf("a changes frame carries no events: %+v", f.Body)
	}
	out := make([]string, 0, len(events))
	for _, one := range events {
		body, _ := one.(map[string]any)
		name, _ := body["type"].(string)
		out = append(out, name)
	}
	return out
}
