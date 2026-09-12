package hub

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

var (
	mapAssetID = testID(20)
	mapGenID   = testID(21)
	charID     = testID(22)
)

func mapRow(owner ulid.ULID) stubRow {
	return stubRow{
		columns: []string{"owner_id", "width", "height", "tile_size", "max_zoom", "tile_gen"},
		values: []driver.Value{
			idValue(owner), int64(4096), int64(4096), int64(512), int64(3), idValue(mapGenID),
		},
	}
}
func characterRow(asset *ulid.ULID) stubRow {
	var image driver.Value
	if asset != nil {
		image = idValue(*asset)
	}
	return stubRow{
		columns: []string{"id", "owner_id", "name", "size", "ac", "current_hp", "max_hp", "initiative_bonus", "asset_id"},
		values: []driver.Value{
			idValue(charID), idValue(playerID), []byte("Ilyana"), []byte("Medium"),
			int64(16), int64(11), int64(14), int64(3), image,
		},
	}
}
func reading(t *testing.T, row stubRow) room.Library {
	t.Helper()
	return stubbedHub(t, row).library(gmID)
}
func refusal(t *testing.T, err error, code string) *room.Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %s refusal, got nothing", code)
	}
	e, ok := err.(*room.Error)
	if !ok {
		t.Fatalf("expected a *room.Error, got %T: %v", err, err)
	}
	if e.Code != code {
		t.Fatalf("code = %s, want %s (%s)", e.Code, code, e.Message)
	}
	return e
}
func TestTheLibraryReadsAMapIntoAReference(t *testing.T) {
	got, err := reading(t, mapRow(gmID)).Map(t.Context(), mapAssetID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := room.MapRef{
		AssetID: mapAssetID, Gen: mapGenID,
		Width: 4096, Height: 4096, TileSize: 512, MaxZoom: 3,
	}
	if got != want {
		t.Fatalf("the reference is %+v, want %+v", got, want)
	}
}
func TestTheLibraryWillNotReadAMapSomebodyElseOwns(t *testing.T) {
	_, err := reading(t, mapRow(playerID)).Map(t.Context(), mapAssetID)
	if got := refusal(t, err, room.CodeNotFound); got.Heading != "Map gone" {
		t.Errorf("heading = %q, want the same one a missing map gives", got.Heading)
	}
}
func TestTheLibraryWillNotReadAMapThatHasNotFinishedTiling(t *testing.T) {
	for name, at := range map[string]int{"no generation": 5, "no width": 1, "no tile size": 3} {
		t.Run(name, func(t *testing.T) {
			row := mapRow(gmID)
			row.values[at] = nil
			_, err := reading(t, row).Map(t.Context(), mapAssetID)
			refusal(t, err, room.CodeInvalid)
		})
	}
}
func TestTheLibraryReadsAMonsterIntoAStatLine(t *testing.T) {
	got, err := reading(t, monsterRow(mapAssetID)).Monster(t.Context(), monsterID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := room.MonsterInfo{
		Name: "Goblin", Size: room.SizeSmall, HP: 7, AC: 15,
		Image: "/assets/images/" + mapAssetID.String(), InitiativeBonus: 2,
	}
	if got != want {
		t.Fatalf("the stat line is %+v, want %+v", got, want)
	}
}
func TestTheLibraryReadsAPictureIntoItsNameAndSize(t *testing.T) {
	got, err := reading(t, tokenRow(mapAssetID, "Ox-drawn wagon")).
		Picture(t.Context(), mapAssetID, room.PictureToken)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := room.PictureInfo{
		Name: "Ox-drawn wagon", Image: "/assets/images/" + mapAssetID.String(),
		Width: 512, Height: 171,
	}
	if got != want {
		t.Fatalf("the picture is %+v, want %+v", got, want)
	}
}
func TestTheLibraryReadsAPictureWithNoSizeAsHavingNone(t *testing.T) {
	row := tokenRow(mapAssetID, "Crate")
	row.values[10], row.values[11] = int64(0), int64(0)
	got, err := reading(t, row).Picture(t.Context(), mapAssetID, room.PictureToken)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Width != 0 || got.Height != 0 {
		t.Fatalf("the picture is %dx%d, want nothing the command can size a pawn by", got.Width, got.Height)
	}
}
func TestTheLibraryReadsACharacterIntoItsStatLine(t *testing.T) {
	got, err := reading(t, characterRow(&mapAssetID)).Character(t.Context(), charID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := room.CharacterInfo{
		ID: charID, OwnerID: playerID, Name: "Ilyana", Size: room.SizeMedium,
		HP: 11, MaxHP: 14, AC: 16, Image: "/assets/images/" + mapAssetID.String(),
		InitiativeBonus: 3,
	}
	if got != want {
		t.Fatalf("the stat line is %+v, want %+v", got, want)
	}
}
func TestTheLibraryReadsACharacterWithNoPortraitAsHavingNone(t *testing.T) {
	got, err := reading(t, characterRow(nil)).Character(t.Context(), charID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Image != "" {
		t.Fatalf("image = %q, want empty so the command can fall back to the account picture", got.Image)
	}
}
func TestEveryLibraryReadIsNotFoundWhenThereIsNoRow(t *testing.T) {
	lib := reading(t, noRows())
	reads := map[string]func() error{
		"a map": func() error {
			_, err := lib.Map(t.Context(), mapAssetID)
			return err
		},
		"a monster": func() error {
			_, err := lib.Monster(t.Context(), monsterID)
			return err
		},
		"a picture": func() error {
			_, err := lib.Picture(t.Context(), mapAssetID, room.PictureAvatar)
			return err
		},
		"a character": func() error {
			_, err := lib.Character(t.Context(), charID)
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			refusal(t, read(), room.CodeNotFound)
		})
	}
}
func TestEveryLibraryReadSaysSoOnAServerWithNoDatabase(t *testing.T) {
	lib := New(nil, Options{Store: &memStore{}}).library(gmID)
	reads := map[string]func() error{
		"a map": func() error {
			_, err := lib.Map(t.Context(), mapAssetID)
			return err
		},
		"a monster": func() error {
			_, err := lib.Monster(t.Context(), monsterID)
			return err
		},
		"a picture": func() error {
			_, err := lib.Picture(t.Context(), mapAssetID, room.PictureToken)
			return err
		},
		"a character": func() error {
			_, err := lib.Character(t.Context(), charID)
			return err
		},
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			refusal(t, read(), room.CodeInvalid)
		})
	}
}

var trackID = testID(23)

func trackRow(name string, uploaded bool) stubRow {
	var at driver.Value
	if uploaded {
		at = time.Unix(0, 0)
	}
	return stubRow{
		columns: []string{
			"id", "owner_id", "journal_id", "file_path", "preview_path", "type",
			"file_name", "size_bytes", "name", "detached_at", "width", "height",
			"tile_size", "max_zoom", "tile_gen", "tile_state", "tile_attempts",
			"tile_lease", "tile_leased_at", "tiled_at", "uploaded_at",
			"created_at", "updated_at",
		},
		values: []driver.Value{
			idValue(trackID), idValue(gmID), nil, []byte("music/x"), nil, []byte("music"),
			[]byte("tavern.mp3"), int64(4 << 20), []byte(name), nil, nil, nil,
			nil, nil, nil, nil, int64(0),
			nil, nil, nil, at,
			time.Unix(0, 0), time.Unix(0, 0),
		},
	}
}
func TestTheLibraryReadsATrackByName(t *testing.T) {
	got, err := reading(t, trackRow("Tavern Brawl", true)).Track(t.Context(), trackID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Name != "Tavern Brawl" {
		t.Fatalf("the track is %+v, want the name the library holds", got)
	}
}
func TestTheLibraryWillNotReadATrackThatNeverFinishedUploading(t *testing.T) {
	_, err := reading(t, trackRow("Tavern Brawl", false)).Track(t.Context(), trackID)
	if got := refusal(t, err, room.CodeNotFound); got.Heading != "Track gone" {
		t.Errorf("heading = %q, want the same one a missing track gives", got.Heading)
	}
}
func TestTheLibraryWillNotReadATrackThatIsNotThere(t *testing.T) {
	_, err := reading(t, noRows()).Track(t.Context(), trackID)
	refusal(t, err, room.CodeNotFound)
}
func stubbedTabletop(t *testing.T, row stubRow) *tabletop {
	t.Helper()
	db := sql.OpenDB(stubConnector{row})
	t.Cleanup(func() { db.Close() })
	store := &memStore{loaded: Loaded{Name: "The Sunless Citadel"}}
	h := New(queries.New(db), Options{
		Store:            store,
		Version:          "test-build",
		SnapshotInterval: time.Hour,
		UnloadGrace:      time.Hour,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		h.Shutdown(ctx)
	})
	return &tabletop{t: t, Hub: h, store: store}
}
func TestASheetSaveDoesNotWakeARoomNobodyIsIn(t *testing.T) {
	h := stubbedHub(t, characterRow(nil))
	h.SyncCharacter(t.Context(), roomID, playerID, charID)
	if h.Loaded() != 0 {
		t.Errorf("a sheet save loaded %d rooms; there is nobody at that table to tell", h.Loaded())
	}
}
func TestASheetSaveReachesTheSeatedPawn(t *testing.T) {
	tb := stubbedTabletop(t, characterRow(nil))
	gm := tb.join(gmID, "Kyle", room.RoleGM)
	layer := activeLayer(t, only(t, gm, "snapshot")[0])
	tb.seat(playerID, charID, "Ari")
	frames(t, gm)
	spawned := seatPawn(charID, "Stale", 5, 5, 10, room.SizeSmall)
	tb.send(gm, "1", &room.PawnSpawn{
		Kind: room.PawnPlayer, Layer: layer, X: 64, Y: 64, Visible: true,
		CharacterID: &charID, Pawn: &spawned,
	})
	id := ulidField(t, onePawn(t, only(t, gm, "pawns.upserted")[0]), "id")
	tb.SyncCharacter(tb.ctx(), roomID, playerID, charID)
	tb.settle()
	p, ok := tb.Pawn(tb.ctx(), roomID, id, room.RoleGM)
	if !ok {
		t.Fatal("the pawn is gone")
	}
	if p.Name != "Ilyana" || p.Size != room.SizeMedium || *p.HP != 11 || *p.MaxHP != 14 || *p.AC != 16 {
		t.Errorf("the pawn is %q %s %d/%d ac %d, want the sheet's row", p.Name, p.Size, *p.HP, *p.MaxHP, *p.AC)
	}
	if got := only(t, gm, "players.upserted", "pawns.upserted"); len(got) != 2 {
		t.Fatalf("the table was told %d things", len(got))
	}
}
func roomWithAStalePawn(t *testing.T) json.RawMessage {
	t.Helper()
	s := room.NewState(roomID, "The Sunless Citadel", room.Env{})
	stale := seatPawn(charID, "Stale", 5, 5, 10, room.SizeSmall)
	stale.ID = testID(70)
	stale.LayerID = s.Table.ActiveLayer
	stale.Visible = true
	s.Pawns = append(s.Pawns, stale)
	blob, err := room.Marshal(s)
	if err != nil {
		t.Fatalf("marshalling the room: %v", err)
	}
	return blob
}
func TestJoiningARoomBringsTheSheetTheTableMissedWhileItSlept(t *testing.T) {
	tb := stubbedTabletop(t, characterRow(nil))
	tb.store.loaded.Snapshot = roomWithAStalePawn(t)
	c := newClient(room.Player{
		ID: playerID, Name: "Ari", Role: room.RolePlayer, CharacterID: &charID,
	}, tb.opts.SendBuffer)
	if err := tb.actor().post(tb.ctx(), join{c: c}); err != nil {
		t.Fatalf("join was refused: %v", err)
	}
	eventually(t, "the pawn to catch up with the sheet", func() bool {
		p, ok := tb.Pawn(tb.ctx(), roomID, testID(70), room.RoleGM)
		return ok && p.Name == "Ilyana" && *p.HP == 11 && *p.MaxHP == 14 && *p.AC == 16
	})
}
func TestAGMJoiningResyncsNobodyElsesCharacter(t *testing.T) {
	tb := stubbedTabletop(t, characterRow(nil))
	tb.store.loaded.Snapshot = roomWithAStalePawn(t)
	tb.join(gmID, "Kyle", room.RoleGM)
	tb.settle()
	p, ok := tb.Pawn(tb.ctx(), roomID, testID(70), room.RoleGM)
	if !ok {
		t.Fatal("the pawn is gone")
	}
	if p.Name != "Stale" || *p.HP != 5 {
		t.Errorf("the GM's arrival rewrote a player's pawn to %q %d", p.Name, *p.HP)
	}
}
