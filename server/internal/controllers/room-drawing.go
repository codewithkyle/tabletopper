package controllers

import (
	"net/http"

	"github.com/oklog/ulid/v2"

	"tabletopper/internal/room"
)

// THE ONE VERB DRAWING HAS THAT IS NOT A GESTURE. Everything else -- the pen,
// the eraser, Ctrl+Z -- happens on the canvas and travels over the socket, and
// each of those acts on one line. This empties a whole floor at once, which is
// not a thing a hand does, so it is a menu item beside Clear blood and Clear
// tabletop rather than a control on the tool's own pill.
//
// IT ACTS ON THE FLOOR THE GM IS LOOKING AT, which is why the layer arrives in
// the form rather than in the path; see viewedLayerCommands, whose whole reason
// for existing is that rule.
//
// IT IS THE GM'S ALONE even though drawing is everybody's. Rubbing out one line
// is the author's own and is refused for anybody else by StrokeErase; throwing
// away every line on the floor, including four other people's, is the room
// owner's. StrokeClear says so and this handler tests nothing itself.

// ClearLayerDrawing wipes one floor's drawing.
//
// IT TAKES THE BLOOD WITH IT, and that is not a side effect to be apologised
// for. Blood decals are client-only -- no event carries them -- and
// stroke.cleared is the only thing that has ever wiped them; main.ts has
// listened for it since phase 5 and table.clear sends one per floor for the
// same reason. "Clear drawing" is what a GM presses when they want the floor
// back, and the marks a fight left on it are part of what they mean.
func (a *App) ClearLayerDrawing(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the drawing", func(layer ulid.ULID) []room.Command {
		return []room.Command{&room.StrokeClear{Layer: layer}}
	})
}
