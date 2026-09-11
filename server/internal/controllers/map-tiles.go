package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/queries"
	"tabletopper/internal/storage"
	"tabletopper/internal/tiler"

	"github.com/oklog/ulid/v2"
)
























const tileExtension = ".webp"



type tileCoords struct {
	gen ulid.ULID
	z   int
	x   int
	y   int
}





func parseTileCoords(gen string, z string, name string) (tileCoords, bool) {
	generation, err := ulid.Parse(gen)
	if err != nil {
		return tileCoords{}, false
	}

	indexes, ok := strings.CutSuffix(name, tileExtension)
	if !ok {
		return tileCoords{}, false
	}
	xs, ys, ok := strings.Cut(indexes, "_")
	if !ok {
		return tileCoords{}, false
	}

	level, ok := parseTileIndex(z)
	if !ok {
		return tileCoords{}, false
	}
	x, ok := parseTileIndex(xs)
	if !ok {
		return tileCoords{}, false
	}
	y, ok := parseTileIndex(ys)
	if !ok {
		return tileCoords{}, false
	}

	return tileCoords{gen: generation, z: level, x: x, y: y}, true
}









func parseTileIndex(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || strconv.Itoa(n) != s {
		return 0, false
	}
	return n, true
}




















func tileExists(row queries.GetMapPyramidRow, tile tileCoords) bool {
	if row.TileGen == nil || *row.TileGen != tile.gen {
		return false
	}
	if !row.Width.Valid || !row.Height.Valid || !row.TileSize.Valid || !row.MaxZoom.Valid {
		return false
	}
	
	
	if tile.z < 0 || tile.x < 0 || tile.y < 0 {
		return false
	}
	if tile.z > int(row.MaxZoom.Int16) {
		return false
	}

	tileSize := int(row.TileSize.Int16)
	return tile.x < tiler.LevelTiles(int(row.Width.Int32), tileSize, tile.z) &&
		tile.y < tiler.LevelTiles(int(row.Height.Int32), tileSize, tile.z)
}







func missingTile(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}













func (a *App) GetMapTile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	assetID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		missingTile(w)
		return
	}

	tile, ok := parseTileCoords(r.PathValue("gen"), r.PathValue("z"), r.PathValue("tile"))
	if !ok {
		missingTile(w)
		return
	}

	row, err := a.Queries.GetMapPyramid(ctx, assetID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load map pyramid row", "error", err, "assetID", assetID.String())
		}
		missingTile(w)
		return
	}

	if !tileExists(row, tile) {
		missingTile(w)
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	a.streamImage(
		w, r,
		storage.MapTileKey(row.OwnerID, assetID, tile.gen, tile.z, tile.x, tile.y),
		fmt.Sprintf(`"%s-%d-%d-%d"`, tile.gen, tile.z, tile.x, tile.y),
	)
}
