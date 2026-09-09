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

// THE SECURITY TEST, AND IT IS FIRST BECAUSE IT IS THE ONE THING IN THIS PHASE
// THAT MUST NOT BE GOT WRONG.
//
// The whole two-audience design says a hidden pawn never reaches a player's
// browser and a monster's exact hit points do not either unless the room says
// so. Every socket emission honours that because Apply projects on the way out.
// The pawn window is a SECOND door into the same state -- an HTTP GET carrying
// a pawn id -- and an unprojected one would be a way round all of it: a player
// guesses a ULID and reads the stat line the socket was careful never to send.
func TestPawnIsProjectedForTheRoleThatAsksForIt(t *testing.T) {
	tb := newTabletop(t, Options{})

	gm := tb.join(gmID, "Kyle", room.RoleGM)
	player := tb.join(playerID, "Ari", room.RolePlayer)
	frames(t, gm)
	frames(t, player)

	tb.send(gm, "opt", &room.TableSetOptions{PawnLabels: room.LabelsDefault, PlayersCanDraw: true})
	frames(t, gm)
	frames(t, player)

	visible := tb.spawnMonster(gm, "Goblin", true, 4, 10)
	hidden := tb.spawnMonster(gm, "Ambusher", false, 9, 10)

	// The GM sees both, whole.
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

	// The player is shown the visible one as a WORD and no numbers.
	p, ok := tb.Pawn(tb.ctx(), roomID, visible, room.RolePlayer)
	if !ok {
		t.Fatal("the player was shown nothing of a visible monster")
	}
	if p.HP != nil || p.MaxHP != nil {
		t.Errorf("the player's copy carries hit points %v/%v; the room labels words", p.HP, p.MaxHP)
	}
	if p.HPBand == nil || *p.HPBand != room.BandBloody {
		t.Errorf("the player's copy band = %v, want bloody for 4 of 10", p.HPBand)
	}

	// AND NO ARMOUR CLASS, which is the half of the projection this door would
	// be the easiest way round: the socket never sends it and a GET that did
	// would hand the party what to roll against.
	if p.AC != nil {
		t.Errorf("the player's copy carries armour class %d", *p.AC)
	}

	// And nothing at all of the hidden one. This is the assertion the door
	// exists for: not a pawn with a flag on it, not an empty stat line -- no
	// answer, indistinguishable from a pawn id that never existed.
	if got, ok := tb.Pawn(tb.ctx(), roomID, hidden, room.RolePlayer); ok || got != nil {
		t.Fatalf("a player asking for a hidden pawn was handed %+v", got)
	}
}

// A pawn on another floor is as absent as a hidden one, and for the same
// reason: shown means visible AND on the active layer, so the GM stepping
// upstairs to prepare takes the whole floor out of the players' reach.
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

// A pawn that is not there is not an error and not a panic: it is the same
// nothing a hidden one is.
func TestPawnIsNothingWhenItIsNotThere(t *testing.T) {
	tb := newTabletop(t, Options{})
	tb.join(gmID, "Kyle", room.RoleGM)

	if _, ok := tb.Pawn(tb.ctx(), roomID, testID(99), room.RoleGM); ok {
		t.Error("a pawn id nobody spawned was answered")
	}
}

// THE WRITE-THROUGH FIRES FOR A PLAYER'S PAWN AND NOTHING ELSE. A monster
// taking damage must not write to a characters row, and a player pawn placed
// from a token -- which has no character behind it -- has nothing to write to.
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

// RESOLUTION: the hub's half of a spawn, which is where a reference becomes a
// pawn. Every case below is one row in, one pawn out.

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

// A PLAYER'S SPAWN RESOLVES INTO NOTHING AND ASKS THE DATABASE NOTHING. Putting
// something on the table is the GM's act -- PawnSpawn.Authorize refuses
// everybody else -- and Authorize runs AFTER resolution, so without the early
// return this half would go and read a manual on behalf of a command that is
// about to be thrown away.
//
// THE STUB IS THE PROOF. It holds the same row that builds a Goblin for the GM
// in the test above; a player gets no pawn out of it and no error either, because
// the refusal is not this half's to give and "not found" would be the wrong
// thing to say about a monster that is right there.
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

// A monster nobody at this table owns is not found rather than forbidden. The
// statement is scoped to the asker, so another GM's manual matches nothing --
// and "not found" is the honest answer, because to this GM it does not exist.
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

// A monster with no picture draws as a disc with its initials, which is an
// empty image rather than a broken one.
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

// THE SIZE COLUMN IS A VARCHAR WRITTEN BY FORMS AND IMPORTERS, so resolution
// normalises rather than refuses. A row somebody would like to put on a table
// is not a bug report.
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

// An object takes the picture's own name when the dialog left the field blank,
// which is what makes placing a wagon one click rather than two.
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

// AN OBJECT IS THE SIZE OF ITS PICTURE, and the spawn command says nothing
// about it. This is the whole of what replaced the two cell-count fields the
// dialog used to carry: the row that holds the picture holds its dimensions,
// and those are what land on the table.
func TestAnObjectIsTheSizeOfItsPicture(t *testing.T) {
	asset := testID(9)
	h := stubbedHub(t, tokenRow(asset, "Ox-drawn wagon"))

	cmd := &room.PawnSpawn{Kind: room.PawnObject, AssetID: &asset}
	if err := h.resolveObject(context.Background(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}

	// tokenRow's picture is 512 by 171, which is the shape a wagon is stored
	// at; nothing about the room's grid comes into it.
	if cmd.Pawn.Width != 512 || cmd.Pawn.Height != 171 {
		t.Errorf("the wagon is %dx%d, want the picture's 512x171", cmd.Pawn.Width, cmd.Pawn.Height)
	}
}

// An object with no picture cannot be placed at all: an object is drawn as its
// image, so one without is a rectangle of nothing.
func TestObjectWithNoAssetIsRefused(t *testing.T) {
	h := stubbedHub(t, noRows())

	cmd := &room.PawnSpawn{Kind: room.PawnObject}
	err := h.resolveObject(context.Background(), roomID, room.Actor{ID: gmID, Role: room.RoleGM}, cmd)

	refusal, ok := err.(*room.Error)
	if !ok || refusal.Code != room.CodeInvalid {
		t.Fatalf("error = %v, want an invalid refusal", err)
	}
}

// A token placed as a creature arrives with the size the dialog chose and a
// placeholder stat line, because a picture has no hit points to read.
func TestResolvingATokenTakesTheSizeFromTheWire(t *testing.T) {
	asset := testID(10)
	h := stubbedHub(t, tokenRow(asset, "Bandit"))

	cmd := &room.PawnSpawn{Kind: room.PawnNPC, AssetID: &asset, Size: room.SizeLarge}
	if err := h.resolveToken(context.Background(), room.Actor{ID: gmID, Role: room.RoleGM}, cmd); err != nil {
		t.Fatalf("resolution failed: %v", err)
	}

	if cmd.Pawn.Size != room.SizeLarge {
		t.Errorf("size = %q, want large", cmd.Pawn.Size)
	}
	if cmd.Pawn.HP == nil || *cmd.Pawn.HP != npcHP || cmd.Pawn.AC == nil || *cmd.Pawn.AC != npcAC {
		t.Errorf("stat line = %v/%v, want the placeholder", cmd.Pawn.HP, cmd.Pawn.AC)
	}
}

// spawnMonster puts one monster on the table and hands back its id, by
// resolving the command the way the hub would and reading the id off the event
// the GM was sent.
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

	for _, f := range frames(tb.t, gm) {
		if f.Type != "pawn.spawned" {
			continue
		}

		pawn, _ := f.Body["pawn"].(map[string]any)
		raw, _ := pawn["id"].(string)
		id, err := ulid.Parse(raw)
		if err != nil {
			tb.t.Fatalf("a spawned pawn had no id: %v", err)
		}

		return id
	}

	tb.t.Fatalf("%s was not spawned", name)

	return ulid.ULID{}
}

// A HUB WITH A DATABASE THAT ANSWERS ONE ROW, which is what the resolution
// tests above need and nothing else in this package does. It is the shape
// oneRowDB has in internal/controllers, kept here rather than shared because
// the two packages test different things with it and a shared stub grows
// options until it dispatches on SQL.
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

func noRows() stubRow { return stubRow{empty: true} }

// THE IDS GO ACROSS AS SIXTEEN RAW BYTES, which is the column type and
// therefore what the driver would hand back. The 26-character text form is how
// a ULID travels in a URL and in JSON, and ulid.Scan refuses it here for the
// same reason the schema does not store it.
func idValue(id ulid.ULID) driver.Value { return append([]byte(nil), id[:]...) }

// monsterRow is GetMonsterForRoom's six columns, in its order.
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

// tokenRow is a whole assets row, which GetLibraryAsset selects with a star. The
// columns are queries.Asset's fields in order, so a migration that adds one
// fails this rather than quietly shifting every value along by a place.
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

type stubConnector struct{ row stubRow }

func (c stubConnector) Connect(context.Context) (driver.Conn, error) { return stubConn{c.row}, nil }
func (c stubConnector) Driver() driver.Driver                        { return nil }

type stubConn struct{ row stubRow }

func (c stubConn) Prepare(string) (driver.Stmt, error) { return stubStmt{c.row}, nil }
func (c stubConn) Close() error                        { return nil }
func (c stubConn) Begin() (driver.Tx, error)           { return nil, io.ErrUnexpectedEOF }

type stubStmt struct{ row stubRow }

func (s stubStmt) Close() error  { return nil }
func (s stubStmt) NumInput() int { return -1 }

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
