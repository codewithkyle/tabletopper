package hub

import (
	"slices"
	"testing"

	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

func secrets(tb *tabletop, user ulid.ULID) []room.Roll {
	tb.t.Helper()
	reply := make(chan any, 1)
	if err := tb.actor().post(tb.ctx(), ask{
		fn:    func(a *actor) any { return slices.Clone(a.secret[user]) },
		reply: reply,
	}); err != nil {
		tb.t.Fatalf("asking the room for its secret rolls: %v", err)
	}
	rolls, _ := (<-reply).([]room.Roll)
	return rolls
}

func TestASecretRollReachesItsRollerAndNobodyElse(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	other := tb.join(otherID, "Rin", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	frames(t, other)
	tb.send(player, "r1", &room.DiceRoll{Expr: "1d20 + 7", Label: "Stealth", Secret: true})
	only(t, player, "rolled")
	only(t, gm)
	only(t, other)
	mine := secrets(tb, playerID)
	if len(mine) != 1 || !mine[0].Secret || mine[0].Label != "Stealth" {
		t.Fatalf("the roller's secret rolls are %+v, want one Stealth roll", mine)
	}
	if got := secrets(tb, gmID); len(got) != 0 {
		t.Fatalf("the GM was handed %d of somebody else's secret rolls", len(got))
	}
	if got := secrets(tb, otherID); len(got) != 0 {
		t.Fatalf("another player was handed %d of somebody else's secret rolls", len(got))
	}
}
func TestTheGMRollsBehindTheScreenToo(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "r1", &room.DiceRoll{Expr: "1d20 - 1", Secret: true})
	only(t, gm, "rolled")
	only(t, player)
	if got := secrets(tb, gmID); len(got) != 1 {
		t.Fatalf("the GM kept %d secret rolls, want 1", len(got))
	}
	if got := secrets(tb, playerID); len(got) != 0 {
		t.Fatalf("a player was handed the GM's secret roll")
	}
}
func TestEverySocketTheRollerHasOpenIsTold(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	laptop := tb.join(playerID, "Ari", room.RolePlayer)
	phone := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, laptop)
	frames(t, phone)
	tb.send(laptop, "r1", &room.DiceRoll{Expr: "1d20", Secret: true})
	only(t, laptop, "rolled")
	only(t, phone, "rolled")
	only(t, gm)
	if got := secrets(tb, playerID); len(got) != 1 {
		t.Fatalf("two sockets recorded %d rolls, want the one that was made", len(got))
	}
}
func TestAnOpenRollGoesToTheTableAndNotTheRing(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(player, "r1", &room.DiceRoll{Expr: "1d20"})
	only(t, player, "rolls.upserted")
	only(t, gm, "rolls.upserted")
	if got := secrets(tb, playerID); len(got) != 0 {
		t.Fatalf("an open roll was kept as a secret as well")
	}
}
func TestTheSecretRingKeepsOnlyTheLastRolls(t *testing.T) {
	tb := newTabletop(t, Options{})
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, player)
	for range room.SecretRollsMax + 5 {
		tb.send(player, "r", &room.DiceRoll{Expr: "1d6", Secret: true})
	}
	got := secrets(tb, playerID)
	if len(got) != room.SecretRollsMax {
		t.Fatalf("the ring holds %d rolls, want %d", len(got), room.SecretRollsMax)
	}
}
func TestAPlayerWhoLeavesTakesTheirSecretsWithThem(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(player, "r1", &room.DiceRoll{Expr: "1d20", Secret: true})
	if got := secrets(tb, playerID); len(got) != 1 {
		t.Fatalf("the roll was not kept in the first place")
	}
	err := tb.Dispatch(tb.ctx(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, &room.PlayerKick{ID: playerID})
	if err != nil {
		t.Fatalf("the kick was refused: %v", err)
	}
	if got := secrets(tb, playerID); len(got) != 0 {
		t.Fatalf("a player who left the room kept %d secret rolls in it", len(got))
	}
}
