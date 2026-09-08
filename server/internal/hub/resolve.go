package hub

import (
	"context"
	"database/sql"
	"errors"

	"tabletopper/internal/room"
)

// RESOLUTION IS THE HUB'S HALF OF THREE COMMANDS. internal/room has no database
// and cannot get one: a monster's name and hit dice, a character's avatar, a
// map's tile geometry are all rows, and a package whose whole value is being a
// pure function of (state, command) cannot go and read them.
//
// So those three commands carry a resolved field that the wire never sets --
// `json:"-"`, so a client cannot fill it in -- and this is what fills it before
// the command reaches the room. It runs on the calling goroutine, which is the
// socket's read loop or the HTTP handler's, so the room never waits on a
// SELECT while a table full of people waits on the room.
//
// TWO OF THE THREE ARE STILL STUBS. The commands exist because the protocol is
// complete; spawning is not built yet, and a GM who finds a way to send one
// gets a message saying so, which is true, rather than a pawn with no name.
func (h *Hub) resolve(ctx context.Context, who room.Actor, cmd room.Command) error {
	switch cmd := cmd.(type) {
	case *room.TableSetLayerMap:
		return h.resolveMap(ctx, who, cmd)

	case *room.PawnSpawn:
		// To fill in when spawning lands: read the monster, character or token
		// asset the command names and build the whole pawn from it -- name,
		// image, hit points, armour class, size.
		return notBuilt("Spawning is not ready", "Putting a pawn on the table is not built yet.")

	case *room.PawnSpawnCharacters:
		// To fill in with the above: one pawn per connected player who joined
		// with a character, which needs both the room's player list and the
		// characters table.
		return notBuilt("Spawning is not ready", "Spawning the party is not built yet.")
	}

	return nil
}

// resolveMap turns "this asset" into the pyramid the renderer fetches tiles
// from: the generation, the pixel size, the tile size and the top level.
//
// THE VALUES ARE COPIED ONTO THE LAYER AND NOT LOOKED UP PER CLIENT. Six
// browsers asking the database for a map's dimensions on every reconnect is six
// queries for four numbers that changed when the GM picked the map; the room
// state is where the table's configuration lives, and this is part of it.
//
// IT IS ALSO WHY Gen IS IN THE STATE. A map that is re-tiled completes into a
// NEW generation at new URLs, and the old one goes on serving until something
// re-resolves. A layer therefore keeps pointing at the pyramid it was given,
// which is the one whose tiles are still in the bucket -- the GM re-picks the
// map to move to the new generation, and until they do nothing 404s.
//
// EVERY REFUSAL HERE IS THE OWNER'S OR THE PYRAMID'S, never the layer's. Whether
// the layer exists and whether the actor may change it are questions for
// Apply and Authorize, which run after this and own them.
//
// THE OWNER IS WHO IS ASKING, not a field on the command. Only the GM may set a
// layer's map and the GM is the room's owner, so the acting actor's id is the
// library being read -- and a command cannot carry an owner it chose for
// itself, which is what a `json:"-"` field would eventually be asked to do.
func (h *Hub) resolveMap(ctx context.Context, who room.Actor, cmd *room.TableSetLayerMap) error {
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

	// THE OWNER CHECK IS HERE AND NOT IN THE HANDLER, because this is the only
	// place that has both the row and the command. A GM may only put their own
	// maps on their own table: the tile route is deliberately unscoped so that
	// everybody AT the table can fetch them, and that is exactly why choosing
	// one has to be scoped.
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

// notBuilt is invalid rather than a code of its own. The client cannot fix it
// by retrying and it is not a permission problem, which is exactly what invalid
// means -- and adding a sixth code for "this release does not do that" would
// put a temporary state in the protocol's permanent vocabulary.
func notBuilt(heading, message string) error {
	return &room.Error{Code: room.CodeInvalid, Heading: heading, Message: message}
}
