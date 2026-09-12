package hub

import (
	"database/sql/driver"
	"testing"

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
		columns: []string{"id", "owner_id", "name", "size", "ac", "current_hp", "max_hp", "asset_id"},
		values: []driver.Value{
			idValue(charID), idValue(playerID), []byte("Ilyana"), []byte("Medium"),
			int64(16), int64(11), int64(14), image,
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
		Image: "/assets/images/" + mapAssetID.String(),
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
