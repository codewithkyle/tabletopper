package hub
import (
	"context"
	"database/sql"
	"errors"
	"tabletopper/internal/room"
	"github.com/oklog/ulid/v2"
)
func (h *Hub) resolve(ctx context.Context, roomID ulid.ULID, who room.Actor, cmd room.Command) error {
	switch cmd := cmd.(type) {
	case *room.TableSetLayerMap:
		return h.resolveMap(ctx, who, cmd)
	case *room.PawnSpawn:
		return h.resolveSpawn(ctx, roomID, who, cmd)
	case *room.PawnSpawnCharacters:
		return h.resolveParty(ctx, roomID, who, cmd)
	}
	return nil
}
func (h *Hub) resolveMap(ctx context.Context, who room.Actor, cmd *room.TableSetLayerMap) error {
	if !who.GM() {
		return nil
	}
	if h.queries == nil {
		return notBuilt("Maps are not ready", "This server cannot read maps.")
	}
	row, err := h.queries.GetMapPyramid(ctx, cmd.AssetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &room.Error{
				Code:    room.CodeNotFound,
				Heading: "Map gone",
				Message: "That map is no longer in your library.",
			}
		}
		return err
	}
	if row.OwnerID != who.ID {
		return &room.Error{
			Code:    room.CodeNotFound,
			Heading: "Map gone",
			Message: "That map is no longer in your library.",
		}
	}
	if row.TileGen == nil || !row.Width.Valid || !row.Height.Valid || !row.TileSize.Valid || !row.MaxZoom.Valid {
		return &room.Error{
			Code:    room.CodeInvalid,
			Heading: "Map not ready",
			Message: "That map has not finished tiling yet. Try again in a moment.",
		}
	}
	cmd.Map = &room.MapRef{
		AssetID:  cmd.AssetID,
		Gen:      *row.TileGen,
		Width:    int(row.Width.Int32),
		Height:   int(row.Height.Int32),
		TileSize: int(row.TileSize.Int16),
		MaxZoom:  int(row.MaxZoom.Int16),
	}
	return nil
}
func notBuilt(heading, message string) error {
	return &room.Error{Code: room.CodeInvalid, Heading: heading, Message: message}
}
