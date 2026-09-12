package hub

import (
	"context"
	"database/sql"
	"errors"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"

	"github.com/oklog/ulid/v2"
)

type library struct {
	q     *queries.Queries
	owner ulid.ULID
}

func (h *Hub) library(owner ulid.ULID) room.Library {
	return library{q: h.queries, owner: owner}
}
func (l library) Map(ctx context.Context, asset ulid.ULID) (room.MapRef, error) {
	if l.q == nil {
		return room.MapRef{}, notBuilt("Maps are not ready", "This server cannot read maps.")
	}
	row, err := l.q.GetMapPyramid(ctx, asset)
	if err != nil {
		return room.MapRef{}, missing(err, "Map gone", "That map is no longer in your library.")
	}
	if row.OwnerID != l.owner {
		return room.MapRef{}, absent("Map gone", "That map is no longer in your library.")
	}
	if row.TileGen == nil || !row.Width.Valid || !row.Height.Valid || !row.TileSize.Valid || !row.MaxZoom.Valid {
		return room.MapRef{}, notBuilt("Map not ready", "That map has not finished tiling yet. Try again in a moment.")
	}
	return room.MapRef{
		AssetID:  asset,
		Gen:      *row.TileGen,
		Width:    int(row.Width.Int32),
		Height:   int(row.Height.Int32),
		TileSize: int(row.TileSize.Int16),
		MaxZoom:  int(row.MaxZoom.Int16),
	}, nil
}
func (l library) Monster(ctx context.Context, id ulid.ULID) (room.MonsterInfo, error) {
	if l.q == nil {
		return room.MonsterInfo{}, notBuilt("Spawning is not ready", "This server cannot read the library.")
	}
	row, err := l.q.GetMonsterForRoom(ctx, queries.GetMonsterForRoomParams{ID: id, OwnerID: l.owner})
	if err != nil {
		return room.MonsterInfo{}, missing(err, "Monster gone", "That monster is no longer in your manual.")
	}
	return room.MonsterInfo{
		Name:            row.Name,
		Size:            room.CreatureSize(row.Size),
		HP:              int(row.HP),
		AC:              int(row.AC),
		Image:           imageURL(row.AssetID),
		InitiativeBonus: int(row.InitiativeBonus),
	}, nil
}
func (l library) Picture(ctx context.Context, id ulid.ULID, kind room.PictureKind) (room.PictureInfo, error) {
	if l.q == nil {
		return room.PictureInfo{}, notBuilt("Spawning is not ready", "This server cannot read the library.")
	}
	asset, err := l.q.GetLibraryAsset(ctx, queries.GetLibraryAssetParams{
		ID:      id,
		OwnerID: l.owner,
		Type:    assetType(kind),
	})
	if err != nil {
		return room.PictureInfo{}, missing(err, "Picture gone", "That picture is no longer in your library.")
	}
	width, height := 0, 0
	if asset.Width.Valid && asset.Height.Valid && asset.Width.Int32 > 0 && asset.Height.Int32 > 0 {
		width, height = int(asset.Width.Int32), int(asset.Height.Int32)
	}
	return room.PictureInfo{
		Name:   asset.Name,
		Image:  imageURL(&asset.ID),
		Width:  width,
		Height: height,
	}, nil
}
func (l library) Character(ctx context.Context, id ulid.ULID) (room.CharacterInfo, error) {
	if l.q == nil {
		return room.CharacterInfo{}, notBuilt("Spawning is not ready", "This server cannot read the roster.")
	}
	row, err := l.q.GetCharacterForRoom(ctx, id)
	if err != nil {
		return room.CharacterInfo{}, missing(err, "Character gone", "That character no longer exists.")
	}
	return room.CharacterInfo{
		ID:              row.ID,
		OwnerID:         row.OwnerID,
		Name:            row.Name,
		Size:            room.CreatureSize(row.Size),
		HP:              int(row.CurrentHP),
		MaxHP:           int(row.MaxHP),
		AC:              int(row.AC),
		Image:           imageURL(row.AssetID),
		InitiativeBonus: int(row.InitiativeBonus),
	}, nil
}
func assetType(kind room.PictureKind) queries.AssetsType {
	if kind == room.PictureAvatar {
		return queries.AssetsTypeAvatar
	}
	return queries.AssetsTypeToken
}
func imageURL(id *ulid.ULID) string {
	if id == nil {
		return ""
	}
	return "/assets/images/" + id.String()
}
func notBuilt(heading, message string) error {
	return &room.Error{Code: room.CodeInvalid, Heading: heading, Message: message}
}
func absent(heading, message string) error {
	return &room.Error{Code: room.CodeNotFound, Heading: heading, Message: message}
}
func missing(err error, heading, message string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return absent(heading, message)
	}
	return err
}
