package hub

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

var monsterID = testID(5)

func TestPawnIsProjectedForTheRoleThatAsksForIt(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)
	tb.send(gm, "opt", &room.TableSetOptions{PawnLabels: room.LabelsDefault, PlayersCanDraw: true, InitiativeGrouping: room.GroupMonsters})
	frames(t, gm)
	frames(t, player)
	visible := tb.spawnMonster(gm, "Goblin", true, 4, 10)
	hidden := tb.spawnMonster(gm, "Ambusher", false, 9, 10)
	for _, id := range []ulid.ULID{visible, hidden} {
		p, ok := tb.Pawn(tb.ctx(), roomID, id, room.RoleGM)
		if !ok {
			t.Fatalf("the GM was shown nothing of pawn %s", id)
		}
		if p.HP == nil || p.MaxHP == nil {
			t.Errorf("the GM's copy of %s lost its hit points", p.Name)
		}
		if p.HPBand != nil {
			t.Errorf("the GM's copy of %s carries a band, which is a projection and should never be in it", p.Name)
		}
	}
	p, ok := tb.Pawn(tb.ctx(), roomID, visible, room.RolePlayer)
	if !ok {
		t.Fatal("the player was shown nothing of a visible monster")
	}
	if p.HP == nil || *p.HP != 4 || p.MaxHP == nil || *p.MaxHP != 10 {
		t.Errorf("the player's copy lost its hit points %v/%v", p.HP, p.MaxHP)
	}
	if p.HPBand == nil || *p.HPBand != room.BandBloody {
		t.Errorf("the player's copy band = %v, want bloody for 4 of 10", p.HPBand)
	}
	if p.AC != nil {
		t.Errorf("the player's copy carries armour class %d", *p.AC)
	}
	if got, ok := tb.Pawn(tb.ctx(), roomID, hidden, room.RolePlayer); ok || got != nil {
		t.Fatalf("a player asking for a hidden pawn was handed %+v", got)
	}
}
func TestPawnIsNothingForAPlayerOnAnotherLayer(t *testing.T) {
	tb := newTabletop(t, Options{})
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	frames(t, gm)
	pawn := tb.spawnMonster(gm, "Guard", true, 11, 11)
	tb.send(gm, "add", &room.TableAddLayer{Name: "First floor"})
	view, ok := tb.Table(tb.ctx(), roomID)
	if !ok || len(view.Table.Layers) != 2 {
		t.Fatalf("the second layer was not added: %+v", view)
	}
	tb.send(gm, "active", &room.TableSetActiveLayer{Layer: view.Table.Layers[1].ID})
	if _, ok := tb.Pawn(tb.ctx(), roomID, pawn, room.RolePlayer); ok {
		t.Error("a player was shown a pawn on a floor that is no longer active")
	}
	if _, ok := tb.Pawn(tb.ctx(), roomID, pawn, room.RoleGM); !ok {
		t.Error("the GM lost sight of a pawn on another floor; it is still on their table")
	}
}
func TestPawnIsNothingWhenItIsNotThere(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)
	if _, ok := tb.Pawn(tb.ctx(), roomID, testID(99), room.RoleGM); ok {
		t.Error("a pawn id nobody spawned was answered")
	}
}
func TestWriteThroughOwesOnlyPlayerPawnsWithASheet(t *testing.T) {
	var mu sync.Mutex
	var written []ulid.ULID
	tb := newTabletop(t, Options{WriteSheet: func(ctx context.Context, character ulid.ULID, v room.SheetVitals) error {
		mu.Lock()
		defer mu.Unlock()
		written = append(written, character)
		return nil
	}})
	sheet, monster, projected := testID(41), testID(42), testID(43)
	hurt, full := 9, 12
	seated := seatPawn(sheet, "Ilyana", hurt, full, 15, room.SizeMedium)
	seated.ID = testID(50)
	beast := seatPawn(monster, "Ogre", hurt, full, 15, room.SizeLarge)
	beast.ID, beast.Kind = testID(51), room.PawnMonster
	crate := seatPawn(projected, "Crate", hurt, full, 15, room.SizeMedium)
	crate.ID, crate.Kind, crate.CharacterID = testID(52), room.PawnObject, nil
	nobody := seatPawn(projected, "Ilyana", hurt, full, 15, room.SizeMedium)
	nobody.ID, nobody.CharacterID = testID(53), nil
	hidden := seatPawn(projected, "Ilyana", hurt, full, 15, room.SizeMedium)
	hidden.ID, hidden.HP = testID(54), nil
	upserted := []room.Pawn{seated, beast, crate, nobody, hidden}
	before := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	for _, p := range upserted {
		if p.HP != nil {
			p.HP = &full
		}
		before.Pawns = append(before.Pawns, p)
	}
	a := tb.actor()
	reply := make(chan any, 1)
	if err := a.post(tb.ctx(), ask{fn: func(a *actor) any {
		a.changed(before, []room.Change{&room.PawnsUpserted{Pawns: upserted}})
		return nil
	}, reply: reply}); err != nil {
		t.Fatalf("post: %v", err)
	}
	<-reply
	a.sheet.stop()
	mu.Lock()
	defer mu.Unlock()
	if len(written) != 1 || written[0] != sheet {
		t.Fatalf("the sheets written were %v, want %s alone: a monster, an object, a pawn with nobody behind it and a pawn whose hit points were projected away owe nothing", written, sheet)
	}
}
func (tb *tabletop) spawnMonster(gm *client, name string, visible bool, hp int, maxHP int) ulid.ULID {
	tb.t.Helper()
	view, ok := tb.Table(tb.ctx(), roomID)
	if !ok {
		tb.t.Fatal("the room would not answer with its table")
	}
	ac := 15
	tb.send(gm, "spawn", &room.PawnSpawn{
		Kind:    room.PawnMonster,
		Layer:   view.Table.ActiveLayer,
		Visible: visible,
		Pawn: &room.Pawn{
			Name:      name,
			Size:      room.SizeSmall,
			HP:        &hp,
			MaxHP:     &maxHP,
			AC:        &ac,
			MonsterID: &monsterID,
		},
	})
	for _, f := range events(tb.t, gm) {
		if f.Type != "pawns.upserted" {
			continue
		}
		raw, _ := onePawn(tb.t, f)["id"].(string)
		id, err := ulid.Parse(raw)
		if err != nil {
			tb.t.Fatalf("a spawned pawn had no id: %v", err)
		}
		return id
	}
	tb.t.Fatalf("%s was not spawned", name)
	return ulid.ULID{}
}

type stubRow struct {
	columns []string
	values  []driver.Value
	empty   bool
}

func stubbedHub(t *testing.T, row stubRow) *Hub {
	t.Helper()
	db := sql.OpenDB(stubConnector{row})
	t.Cleanup(func() { db.Close() })
	return New(queries.New(db), Options{Store: &memStore{}, Version: "test-build"})
}
func noRows() stubRow                   { return stubRow{empty: true} }
func idValue(id ulid.ULID) driver.Value { return append([]byte(nil), id[:]...) }
func monsterRow(asset ulid.ULID) stubRow {
	var image driver.Value
	if asset.Compare(ulid.ULID{}) != 0 {
		image = idValue(asset)
	}
	return stubRow{
		columns: []string{"id", "name", "size", "ac", "hp", "initiative_bonus", "asset_id"},
		values:  []driver.Value{idValue(monsterID), []byte("Goblin"), []byte("small"), int64(15), int64(7), int64(2), image},
	}
}
func tokenRow(id ulid.ULID, name string) stubRow {
	return stubRow{
		columns: []string{
			"id", "owner_id", "journal_id", "file_path", "preview_path", "type",
			"file_name", "size_bytes", "name", "detached_at", "width", "height",
			"tile_size", "max_zoom", "tile_gen", "tile_state", "tile_attempts",
			"tile_lease", "tile_leased_at", "tiled_at", "uploaded_at",
			"created_at", "updated_at",
		},
		values: []driver.Value{
			idValue(id), idValue(gmID), nil, []byte("tokens/x.webp"), nil, []byte("token"),
			[]byte("wagon.png"), int64(4096), []byte(name), nil, int64(512), int64(171),
			nil, nil, nil, nil, int64(0),
			nil, nil, nil, nil,
			time.Unix(0, 0), time.Unix(0, 0),
		},
	}
}
func avatarRow(id ulid.ULID, name string) stubRow {
	row := tokenRow(id, name)
	row.values[3] = []byte("avatars/x.webp")
	row.values[5] = []byte("avatar")
	row.values[6] = []byte("face.png")
	row.values[10], row.values[11] = int64(256), int64(256)
	return row
}

type stubConnector struct{ row stubRow }

func (c stubConnector) Connect(context.Context) (driver.Conn, error) { return stubConn{c.row}, nil }
func (c stubConnector) Driver() driver.Driver                        { return nil }

type stubConn struct{ row stubRow }

func (c stubConn) Prepare(string) (driver.Stmt, error) { return stubStmt{c.row}, nil }
func (c stubConn) Close() error                        { return nil }
func (c stubConn) Begin() (driver.Tx, error)           { return nil, io.ErrUnexpectedEOF }

type stubStmt struct{ row stubRow }

func (s stubStmt) Close() error                               { return nil }
func (s stubStmt) NumInput() int                              { return -1 }
func (s stubStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (s stubStmt) Query([]driver.Value) (driver.Rows, error) {
	return &stubRows{row: s.row, done: s.row.empty}, nil
}

type stubRows struct {
	row  stubRow
	done bool
}

func (r *stubRows) Columns() []string { return r.row.columns }
func (r *stubRows) Close() error      { return nil }
func (r *stubRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	copy(dest, r.row.values)
	return nil
}
