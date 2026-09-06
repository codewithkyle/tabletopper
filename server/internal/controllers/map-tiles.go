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

// One tile of one generation of one map's pyramid:
//
//	GET /assets/maps/{id}/tiles/{gen}/{z}/{x}_{y}.webp
//
// It is not a fragment. The prefix marks partial HTML and this answers with
// image/webp, so it sits with the resource it belongs to like the two routes
// under /assets/images.
//
// RequireSessionOr404, like every other image route -- a redirect to the
// sign-in page renders as a broken image rather than as a sign-in page -- and
// unscoped to the owner for the reason GetImage is unscoped: the DM's map has
// to be fetchable by everyone at the table. Ownership gates the writes.
//
// THE LAST SEGMENT IS ONE WILDCARD RATHER THAN TWO. A ServeMux wildcard must be
// a whole path segment, so registering the pattern with "{x}_{y}.webp" in it
// does not merely fail to match -- it panics at registration and the server
// never starts: `bad wildcard segment (must end with '}')`. The segment is
// taken whole and split here.

// tileExtension is the suffix every tile URL carries. It is in the URL rather
// than left to the Content-Type because the renderer's fetch and the browser's
// cache both key on the path, and a path that looks like an image is what a
// person reading a network log expects to see.
const tileExtension = ".webp"

// tileCoords is everything a tile URL names beyond the map itself: which
// generation of the pyramid, which level of it, and which tile of that level.
type tileCoords struct {
	gen ulid.ULID
	z   int
	x   int
	y   int
}

// parseTileCoords reads the three path segments that identify a tile. It
// answers false for anything it does not fully understand, and the caller's
// only move is a 404 -- nothing here is worth an error message, because the
// only caller is a renderer asking for a tile by a URL this server generated.
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

// parseTileIndex reads one non-negative index and insists on the one spelling
// of it.
//
// THE ROUND TRIP IS THE POINT. Atoi alone accepts "007", "+7" and " 7" as
// seven, which would be four URLs for one tile -- and these URLs are cached for
// a year and never revalidated, so a duplicate is a duplicate in every browser
// and every proxy between here and the table. Comparing the number back against
// the text it came from leaves exactly one form of each index.
func parseTileIndex(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || strconv.Itoa(n) != s {
		return 0, false
	}
	return n, true
}

// tileExists decides whether the pyramid the row describes contains the tile
// that was asked for. It is the whole of the validation, and it is separate
// from the handler because it is the part worth testing: a bug here either
// hands out 404s for tiles that are in the bucket or sends R2 a key nothing
// wrote.
//
// EVERY NUMBER IS CHECKED AGAINST THE ROW, none against another URL. The grid
// comes from width, height and tile_size through the same functions the tiler
// built the pyramid with, which is why they live in internal/tiler rather than
// being written out here.
//
// A ROW WITH NO SERVING GENERATION HAS NO TILES. tile_gen is NULL until the
// first build completes, and the four pyramid columns are written in the same
// statement that sets it, so any one of them missing means there is nothing to
// serve and everything is a 404.
//
// tile_state is deliberately absent. A map being re-tiled is 'working' while
// its previous generation is still in the bucket and still named by tile_gen,
// and it goes on serving that generation until the new one is complete.
func tileExists(row queries.GetMapPyramidRow, tile tileCoords) bool {
	if row.TileGen == nil || *row.TileGen != tile.gen {
		return false
	}
	if !row.Width.Valid || !row.Height.Valid || !row.TileSize.Valid || !row.MaxZoom.Valid {
		return false
	}
	// The URL parser has already refused a negative, but this function is the
	// one that decides, so it decides completely.
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

// missingTile answers every failure this route has, and answers all of them the
// same way: the status and nothing else.
//
// NOT http.NotFound, which writes "404 page not found" into a response the
// caller is treating as an image. The caller is a renderer's fetch or an <img>,
// neither of which has anywhere to put a sentence.
func missingTile(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

// GetMapTile serves one tile out of R2.
//
// TILES ARE IMMUTABLE AND CACHED AS SUCH, which is the one place this app's
// caching departs from private, no-cache. An avatar or a preview can be
// replaced at its URL, which is why serveImage revalidates against an
// updated_at ETag. A tile cannot: the generation is in the path, a replaced map
// completes into a new generation at new paths, and a re-tile at a different
// tile size does the same. The bytes behind a tile URL never change, so a
// player panning back over ground they have already crossed makes no request at
// all.
//
// private, because a session is what decides whether there is an answer.
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
