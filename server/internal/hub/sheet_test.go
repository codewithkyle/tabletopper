package hub

import (
	"context"
	"sync"
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func seatVitals(hp, maxHP, ac int, size room.Size) room.SheetVitals {
	return room.SheetVitals{HP: hp, MaxHP: maxHP, AC: ac, Size: size}
}
func seatPawn(character ulid.ULID, name string, hp, maxHP, ac int, size room.Size) room.Pawn {
	return room.Pawn{
		Kind:        room.PawnPlayer,
		CharacterID: &character,
		Name:        name,
		Size:        size,
		HP:          &hp,
		MaxHP:       &maxHP,
		AC:          &ac,
	}
}

func TestSheetWritesLandOneAtATimeAndTheLatestWins(t *testing.T) {
	var mu sync.Mutex
	var written []room.SheetVitals
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	writer := newSheetWriter(func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		first := len(written) == 0
		written = append(written, v)
		mu.Unlock()
		if first {
			started <- struct{}{}
			<-release
		}
		return nil
	})
	writer.put(testID(9), seatVitals(23, 23, 15, room.SizeMedium))
	<-started
	writer.put(testID(9), seatVitals(20, 23, 15, room.SizeMedium))
	writer.put(testID(9), seatVitals(16, 23, 15, room.SizeMedium))
	close(release)
	writer.stop()
	mu.Lock()
	defer mu.Unlock()
	want := []room.SheetVitals{
		seatVitals(23, 23, 15, room.SizeMedium),
		seatVitals(16, 23, 15, room.SizeMedium),
	}
	if len(written) != 2 || written[0] != want[0] || written[1] != want[1] {
		t.Errorf("writes = %v, want %v: one in flight, then the latest", written, want)
	}
}
func TestOnlyAChangedSeatReachesTheSheet(t *testing.T) {
	var mu sync.Mutex
	var writes []room.SheetVitals
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		writes = append(writes, v)
		mu.Unlock()
		started <- struct{}{}
		<-release
		return nil
	}})
	t.Cleanup(func() { close(release) })
	a := tb.actor()
	character := testID(9)
	pawn := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
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
	*pawn.HP = 9
	through(pawn)
	<-started
	release <- struct{}{}
	pawn.Size = room.SizeLarge
	through(pawn)
	<-started
	release <- struct{}{}
	mu.Lock()
	defer mu.Unlock()
	want := []room.SheetVitals{
		seatVitals(12, 12, 15, room.SizeMedium),
		seatVitals(9, 12, 15, room.SizeMedium),
		seatVitals(9, 12, 15, room.SizeLarge),
	}
	if len(writes) != len(want) || writes[0] != want[0] || writes[1] != want[1] || writes[2] != want[2] {
		t.Errorf("writes = %v, want %v: the rename cost nothing and the size did not", writes, want)
	}
}
func TestSpawningAPlayerPawnWritesNothingToTheSheet(t *testing.T) {
	var mu sync.Mutex
	var writes []room.SheetVitals
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		writes = append(writes, v)
		mu.Unlock()
		return nil
	}})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	character := testID(23)
	tb.seat(playerID, character, "Ari")
	spawned := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &character, Pawn: &spawned,
	})
	tb.actor().sheet.stop()
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("writes = %v; a pawn that has just arrived carries the sheet's own values", writes)
	}
}
func TestAChangeFromACommandReachesTheSheetAndARenameDoesNot(t *testing.T) {
	var mu sync.Mutex
	var writes []room.SheetVitals
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		writes = append(writes, v)
		mu.Unlock()
		started <- struct{}{}
		<-release
		return nil
	}})
	t.Cleanup(func() { close(release) })
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	character := testID(21)
	tb.seat(playerID, character, "Ari")
	frames(t, gm)
	spawned := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &character, Pawn: &spawned,
	})
	pawn := ulidField(t, onePawn(t, only(t, gm, "pawns.upserted")[0]), "id")
	hurt := 9
	tb.send(gm, "2", &room.PawnUpdate{ID: pawn, HP: &hurt})
	<-started
	release <- struct{}{}
	name := "Ilyana the Bold"
	tb.send(gm, "3", &room.PawnUpdate{ID: pawn, Name: &name})
	armoured, huge := 18, room.SizeHuge
	tb.send(gm, "4", &room.PawnUpdate{ID: pawn, AC: &armoured, Size: &huge})
	<-started
	release <- struct{}{}
	mu.Lock()
	defer mu.Unlock()
	want := []room.SheetVitals{
		seatVitals(9, 12, 15, room.SizeMedium),
		seatVitals(9, 12, 18, room.SizeHuge),
	}
	if len(writes) != len(want) || writes[0] != want[0] || writes[1] != want[1] {
		t.Errorf("writes = %v, want %v: the armour and the size owe the sheet as much as the wound", writes, want)
	}
}
func TestAChangeFromTheHTTPFormReachesTheSheet(t *testing.T) {
	var mu sync.Mutex
	var writes []room.SheetVitals
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		defer mu.Unlock()
		writes = append(writes, v)
		return nil
	}})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	character := testID(24)
	tb.seat(playerID, character, "Ari")
	frames(t, gm)
	spawned := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &character, Pawn: &spawned,
	})
	pawn := ulidField(t, onePawn(t, only(t, gm, "pawns.upserted")[0]), "id")
	who := room.Actor{ID: gmID, Role: room.RoleGM}
	hurt := 7
	if err := tb.Dispatch(tb.ctx(), roomID, who, &room.PawnUpdate{ID: pawn, HP: &hurt}); err != nil {
		t.Fatalf("the form was refused: %v", err)
	}
	tb.actor().sheet.stop()
	mu.Lock()
	defer mu.Unlock()
	want := seatVitals(7, 12, 15, room.SizeMedium)
	if len(writes) != 1 || writes[0] != want {
		t.Errorf("writes = %v, want [%v]: a change nobody sent over a socket owes the sheet just the same", writes, want)
	}
}
func TestAPawnWithNoSeatOwesTheSheetNothing(t *testing.T) {
	var mu sync.Mutex
	var writes []room.SheetVitals
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		defer mu.Unlock()
		writes = append(writes, v)
		return nil
	}})
	a := tb.actor()
	character := testID(31)
	monster := seatPawn(character, "Ogre", 12, 12, 15, room.SizeLarge)
	monster.Kind = room.PawnMonster
	seatless := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
	seatless.CharacterID = nil
	unarmoured := seatPawn(character, "Ilyana", 12, 12, 15, room.SizeMedium)
	unarmoured.AC = nil
	for _, p := range []room.Pawn{monster, seatless, unarmoured} {
		reply := make(chan any, 1)
		if err := a.post(tb.ctx(), ask{fn: func(a *actor) any { a.writeThrough(p); return nil }, reply: reply}); err != nil {
			t.Fatalf("post: %v", err)
		}
		<-reply
	}
	tb.actor().sheet.stop()
	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("writes = %v; none of those pawns sits in a seat the sheet knows about", writes)
	}
}
