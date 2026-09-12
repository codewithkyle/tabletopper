package hub

import (
	"context"
	"sync"
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func TestSheetWritesLandOneAtATimeAndTheLatestWins(t *testing.T) {
	var mu sync.Mutex
	var written []int
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	writer := newSheetWriter(func(ctx context.Context, character ulid.ULID, hp int) error {
		mu.Lock()
		first := len(written) == 0
		written = append(written, hp)
		mu.Unlock()
		if first {
			started <- struct{}{}
			<-release
		}
		return nil
	})
	writer.put(testID(9), 23)
	<-started
	writer.put(testID(9), 20)
	writer.put(testID(9), 16)
	close(release)
	writer.stop()
	mu.Lock()
	defer mu.Unlock()
	if len(written) != 2 || written[0] != 23 || written[1] != 16 {
		t.Errorf("writes = %v, want [23 16]: one in flight, then the latest", written)
	}
}
func TestOnlyAChangedHitPointTotalReachesTheSheet(t *testing.T) {
	var mu sync.Mutex
	var writes []int
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	tb := newTabletop(t, Options{WriteHP: func(ctx context.Context, character ulid.ULID, hp int) error {
		mu.Lock()
		writes = append(writes, hp)
		mu.Unlock()
		started <- struct{}{}
		<-release
		return nil
	}})
	t.Cleanup(func() { close(release) })
	a := tb.actor()
	character := testID(9)
	hp := 12
	pawn := room.Pawn{Kind: room.PawnPlayer, CharacterID: &character, HP: &hp, Name: "Ilyana"}
	through := func(p room.Pawn) {
		reply := make(chan any, 1)
		if err := a.post(tb.ctx(), ask{fn: func(a *actor) any { a.writeThrough(p); return nil }, reply: reply}); err != nil {
			t.Fatalf("post: %v", err)
		}
		<-reply
	}
	through(pawn)
	<-started
	release <- struct{}{}
	pawn.Name = "Ilyana the Bold"
	through(pawn)
	hp = 9
	through(pawn)
	<-started
	release <- struct{}{}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 || writes[0] != 12 || writes[1] != 9 {
		t.Errorf("writes = %v, want [12 9]: the rename cost nothing", writes)
	}
}

func TestSpawningAPlayerPawnWritesNothingToTheSheet(t *testing.T) {
	var mu sync.Mutex
	var writes []int
	tb := newTabletop(t, Options{WriteHP: func(ctx context.Context, character ulid.ULID, hp int) error {
		mu.Lock()
		writes = append(writes, hp)
		mu.Unlock()
		return nil
	}})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	character := testID(22)
	full := 12
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &character,
		Pawn: &room.Pawn{
			Name: "Ilyana", Size: room.SizeMedium,
			HP: &full, MaxHP: &full, CharacterID: &character,
		},
	})
	tb.actor().sheet.stop()
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("writes = %v; a pawn that has just arrived carries the sheet's own hit points", writes)
	}
}
func TestAHitPointChangeFromACommandReachesTheSheetAndARenameDoesNot(t *testing.T) {
	var mu sync.Mutex
	var writes []int
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	tb := newTabletop(t, Options{WriteHP: func(ctx context.Context, character ulid.ULID, hp int) error {
		mu.Lock()
		writes = append(writes, hp)
		mu.Unlock()
		started <- struct{}{}
		<-release
		return nil
	}})
	t.Cleanup(func() { close(release) })
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	character := testID(21)
	full, hurt, worse := 12, 9, 5
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &character,
		Pawn: &room.Pawn{
			Name: "Ilyana", Size: room.SizeMedium,
			HP: &full, MaxHP: &full, CharacterID: &character,
		},
	})
	pawn := ulidField(t, onePawn(t, only(t, gm, "pawns.upserted")[0]), "id")
	tb.send(gm, "2", &room.PawnUpdate{ID: pawn, HP: &hurt})
	<-started
	release <- struct{}{}
	name := "Ilyana the Bold"
	tb.send(gm, "3", &room.PawnUpdate{ID: pawn, Name: &name})
	tb.send(gm, "4", &room.PawnUpdate{ID: pawn, HP: &worse})
	<-started
	release <- struct{}{}
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 || writes[0] != hurt || writes[1] != worse {
		t.Errorf("writes = %v, want [9 5]: the change went through the derived event and the rename did not", writes)
	}
}
