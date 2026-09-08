package hub

import (
	"context"

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
// ALL THREE ARE STUBS TODAY AND ANSWER invalid. The commands exist because the
// protocol is complete; the features that put a map on a layer and a pawn on
// the table are not built yet. A GM who finds a way to send one gets a message
// saying so, which is true, rather than a pawn with no name.
func (h *Hub) resolve(ctx context.Context, cmd room.Command) error {
	switch cmd.(type) {
	case *room.TableSetLayerMap:
		// To fill in when maps land: read the assets row, refuse one that has
		// not finished tiling, and set Map to the asset id, tile generation,
		// pixel size, tile size and maximum zoom.
		return notBuilt("Maps are not ready", "Putting a map on a layer is not built yet.")

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

// notBuilt is invalid rather than a code of its own. The client cannot fix it
// by retrying and it is not a permission problem, which is exactly what invalid
// means -- and adding a sixth code for "this release does not do that" would
// put a temporary state in the protocol's permanent vocabulary.
func notBuilt(heading, message string) error {
	return &room.Error{Code: room.CodeInvalid, Heading: heading, Message: message}
}
