package hub

import (
	"context"
	"sync"
	"testing"
	"time"

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

	tb := newTabletop(t, Options{WriteHP: func(ctx context.Context, character ulid.ULID, hp int) error {
		mu.Lock()
		defer mu.Unlock()
		writes = append(writes, hp)

		return nil
	}})
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
	pawn.Name = "Ilyana the Bold"
	through(pawn)
	hp = 9
	through(pawn)

	eventually(t, "the sheet writes", func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(writes) >= 2
	})
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 || writes[0] != 12 || writes[1] != 9 {
		t.Errorf("writes = %v, want [12 9]: the rename cost nothing", writes)
	}
}
