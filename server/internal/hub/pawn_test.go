package hub

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
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
	character := testID(7)
	hp := 12
	cases := []struct {
		name string
		pawn room.Pawn
		want bool
	}{
		{
			name: "a player pawn with a sheet",
			pawn: room.Pawn{Kind: room.PawnPlayer, CharacterID: &character, HP: &hp},
			want: true,
		},
		{
			name: "a monster",
			pawn: room.Pawn{Kind: room.PawnMonster, CharacterID: &character, HP: &hp},
			want: false,
		},
		{
			name: "an object",
			pawn: room.Pawn{Kind: room.PawnObject, HP: &hp},
			want: false,
		},
		{
			name: "a player pawn with no sheet behind it",
			pawn: room.Pawn{Kind: room.PawnPlayer, HP: &hp},
			want: false,
		},
		{
			name: "a player pawn whose hit points were projected away",
			pawn: room.Pawn{Kind: room.PawnPlayer, CharacterID: &character},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, value, owed := writeThroughHP(tc.pawn)
			if owed != tc.want {
				t.Fatalf("owed = %v, want %v", owed, tc.want)
			}
			if owed && (id != character || value != hp) {
				t.Errorf("owed a write of %d to %s, want %d to %s", value, id, hp, character)
			}
		})
	}
}
func TestResolvingAMonsterReadsTheGMsManual(t *testing.T) {
	asset := testID(8)
	h := stubbedHub(t, monsterRow(asset))
	cmd := &room.PawnSpawn{Kind: room.PawnMonster, MonsterID: &monsterID}
	if err := h.resolveMonster(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	got := cmd.Pawn
	if got.Name != "Goblin" {
		t.Errorf("name = %q, want Goblin", got.Name)
	}
	if got.Size != room.SizeSmall {
		t.Errorf("size = %q, want small", got.Size)
	}
	if got.HP == nil || *got.HP != 7 || got.MaxHP == nil || *got.MaxHP != 7 {
		t.Errorf("hit points = %v/%v, want 7/7 -- a fresh instance starts undamaged", got.HP, got.MaxHP)
	}
	if got.AC == nil || *got.AC != 15 {
		t.Errorf("armour class = %v, want 15", got.AC)
	}
	if got.Image != "/assets/images/"+asset.String() {
		t.Errorf("image = %q, want the unscoped image route", got.Image)
	}
	if got.MonsterID == nil || *got.MonsterID != monsterID {
		t.Error("the pawn lost the monster it came from, which is what the stat block reads")
	}
}
func TestAPlayersSpawnIsResolvedIntoNothing(t *testing.T) {
	h := stubbedHub(t, monsterRow(testID(8)))
	cmd := &room.PawnSpawn{Kind: room.PawnMonster, MonsterID: &monsterID}
	who := room.Actor{ID: playerID, Role: room.RolePlayer}
	if err := h.resolveSpawn(context.Background(), testID(1), who, cmd); err != nil {
		t.Fatalf("a player's spawn was refused during resolution: %v", err)
	}
	if cmd.Pawn != nil {
		t.Errorf("a player's spawn built %+v; the refusal belongs to Authorize", cmd.Pawn)
	}
}
func TestResolvingAnotherGMsMonsterIsNotFound(t *testing.T) {
	h := stubbedHub(t, noRows())
	cmd := &room.PawnSpawn{Kind: room.PawnMonster, MonsterID: &monsterID}
	err := h.resolveMonster(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd)
	refusal, ok := err.(*room.Error)
	if !ok || refusal.Code != room.CodeNotFound {
		t.Fatalf("error = %v, want a not_found refusal", err)
	}
	if cmd.Pawn != nil {
		t.Error("a refused resolution still built a pawn")
	}
}
func TestResolvingAMonsterWithNoPictureLeavesTheImageEmpty(t *testing.T) {
	row := monsterRow(ulid.ULID{})
	row.values[5] = nil
	h := stubbedHub(t, row)
	cmd := &room.PawnSpawn{Kind: room.PawnMonster, MonsterID: &monsterID}
	if err := h.resolveMonster(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if cmd.Pawn.Image != "" {
		t.Errorf("image = %q, want empty", cmd.Pawn.Image)
	}
}
func TestACharacterPawnFallsBackFromPortraitToAccountToNothing(t *testing.T) {
	portrait := ulid.MustParse("01BX5ZZKBKACTAV9WEVGEMMVWX")
	cases := map[string]struct {
		asset *ulid.ULID
		seat  *room.Player
		want  string
	}{
		"the character's own portrait wins": {
			asset: &portrait,
			seat:  &room.Player{Avatar: "https://img.clerk.com/kyle"},
			want:  "/assets/images/" + portrait.String(),
		},
		"no portrait falls back to the account picture": {
			seat: &room.Player{Avatar: "https://img.clerk.com/kyle"},
			want: "https://img.clerk.com/kyle",
		},
		"the shared placeholder is refused": {
			seat: &room.Player{Avatar: room.DefaultAvatar},
			want: "",
		},
		"a portrait still wins over the placeholder": {
			asset: &portrait,
			seat:  &room.Player{Avatar: room.DefaultAvatar},
			want:  "/assets/images/" + portrait.String(),
		},
		"a character belonging to nobody at the table has no seat to ask": {
			want: "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			row := queries.GetCharacterForRoomRow{Name: "Ilyana", Size: "medium", AssetID: tc.asset}
			if got := characterPawn(row, tc.seat).Image; got != tc.want {
				t.Errorf("image = %q, want %q", got, tc.want)
			}
		})
	}
}
func TestCreatureSizeNormalises(t *testing.T) {
	cases := map[string]room.Size{
		"large":        room.SizeLarge,
		"Large":        room.SizeLarge,
		"  GARGANTUAN": room.SizeGargantuan,
		"":             room.SizeMedium,
		"enormous":     room.SizeMedium,
	}
	for in, want := range cases {
		if got := creatureSize(in); got != want {
			t.Errorf("creatureSize(%q) = %q, want %q", in, got, want)
		}
	}
}
func TestObjectTakesTheAssetsNameWhenBlank(t *testing.T) {
	asset := testID(9)
	h := stubbedHub(t, tokenRow(asset, "Ox-drawn wagon"))
	cmd := &room.PawnSpawn{Kind: room.PawnObject, AssetID: &asset}
	if err := h.resolveObject(context.Background(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if cmd.Pawn.Name != "Ox-drawn wagon" {
		t.Errorf("name = %q, want the asset's own", cmd.Pawn.Name)
	}
	if cmd.Pawn.HP != nil || cmd.Pawn.AC != nil || cmd.Pawn.Conditions != nil {
		t.Error("an object was given a stat line; it has a size and a picture and nothing else")
	}
}
func TestAnObjectIsTheSizeOfItsPicture(t *testing.T) {
	asset := testID(9)
	h := stubbedHub(t, tokenRow(asset, "Ox-drawn wagon"))
	cmd := &room.PawnSpawn{Kind: room.PawnObject, AssetID: &asset}
	if err := h.resolveObject(context.Background(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if cmd.Pawn.Width != 512 || cmd.Pawn.Height != 171 {
		t.Errorf("the wagon is %dx%d, want the picture's 512x171", cmd.Pawn.Width, cmd.Pawn.Height)
	}
}
func TestObjectWithNoAssetIsRefused(t *testing.T) {
	h := stubbedHub(t, noRows())
	cmd := &room.PawnSpawn{Kind: room.PawnObject}
	err := h.resolveObject(context.Background(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd)
	refusal, ok := err.(*room.Error)
	if !ok || refusal.Code != room.CodeInvalid {
		t.Fatalf("error = %v, want an invalid refusal", err)
	}
}
func TestResolvingAnNPCTakesTheStatLineFromTheWire(t *testing.T) {
	asset := testID(10)
	h := stubbedHub(t, avatarRow(asset, "Bandit"))
	hp, maxHP, ac := 9, 12, 13
	cmd := &room.PawnSpawn{
		Kind:    room.PawnNPC,
		AssetID: &asset,
		Size:    room.SizeLarge,
		HP:      &hp,
		MaxHP:   &maxHP,
		AC:      &ac,
	}
	if err := h.resolveNPC(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if cmd.Pawn.Size != room.SizeLarge {
		t.Errorf("size = %q, want large", cmd.Pawn.Size)
	}
	if cmd.Pawn.HP == nil || *cmd.Pawn.HP != hp {
		t.Errorf("hit points = %v, want %d", cmd.Pawn.HP, hp)
	}
	if cmd.Pawn.MaxHP == nil || *cmd.Pawn.MaxHP != maxHP {
		t.Errorf("maximum hit points = %v, want %d", cmd.Pawn.MaxHP, maxHP)
	}
	if cmd.Pawn.AC == nil || *cmd.Pawn.AC != ac {
		t.Errorf("armour class = %v, want %d", cmd.Pawn.AC, ac)
	}
}
func TestResolvingAnNPCWithNoStatLineFallsBackToThePlaceholder(t *testing.T) {
	asset := testID(11)
	h := stubbedHub(t, avatarRow(asset, "Innkeeper"))
	cmd := &room.PawnSpawn{Kind: room.PawnNPC, AssetID: &asset, Size: room.SizeMedium}
	if err := h.resolveNPC(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}
	if cmd.Pawn.HP == nil || *cmd.Pawn.HP != npcHP || cmd.Pawn.AC == nil || *cmd.Pawn.AC != npcAC {
		t.Errorf("stat line = %v/%v, want the placeholder", cmd.Pawn.HP, cmd.Pawn.AC)
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
		columns: []string{"id", "name", "size", "ac", "hp", "asset_id"},
		values:  []driver.Value{idValue(monsterID), []byte("Goblin"), []byte("small"), int64(15), int64(7), image},
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
