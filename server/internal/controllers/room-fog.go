package controllers

import (
	"net/http"

	"github.com/oklog/ulid/v2"

	"tabletopper/internal/room"
)

// THE FOG MENU'S TWO VERBS. Everything else about fog happens on the canvas and
// travels over the socket; these two are here because they are menu items, and
// a menu item is an htmx button.
//
// THE LAYER IS A FORM VALUE AND NOT A PATH SEGMENT, which is the one thing
// about these routes worth explaining. Both act on the floor the GM is LOOKING
// at rather than the floor the players are on -- covering the first floor while
// the party is still in the cellar is the case the feature exists for -- and
// which floor that is exists only in the browser: the viewed layer is a local
// override that is never sent, for the reason mountLayerBar spells out. So the
// room bundle writes it into hx-vals before each request. It cannot write it
// into the path: htmx captures a path when it processes an element, so an
// attribute rewritten afterwards is ignored, which is the same wall the layer
// bar hit and answered with htmx.ajax.
//
// BOTH ARE POST, including the one that empties something. A menu item is a
// button carrying hx-post and a second verb would be a second branch in
// roomBarItem for one route.
//
// BOTH THROW AWAY EVERY SHAPE ON THE FLOOR AND NEITHER CAN BE UNDONE, so both
// carry hx-confirm and both are one click from a modal that says what goes. The
// only undo in this feature is Ctrl+Z on the canvas, which takes back the newest
// shape and cannot help once the collection is gone.

// FillLayerFog covers a floor: every shape on it goes, the floor is prefilled,
// and its fog is on. What a GM means by "fill fog" is a floor they can uncover
// from scratch, so the shapes have to go with it -- a floor left covered with
// last session's reveals still cut out of it is not filled.
//
// THE THREE COMMANDS ARE IN THIS ORDER for one reason: the clear goes first, so
// that no client ever sees a covered floor with the old holes still in it. Every
// one of them is a separate broadcast and the party is watching.
func (a *App) FillLayerFog(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "fill the fog", func(layer ulid.ULID) []room.Command {
		return []room.Command{
			&room.FogClear{Layer: layer},
			&room.FogSetPrefill{Layer: layer, Prefill: true},
			&room.FogSetEnabled{Layer: layer, Enabled: true},
		}
	})
}

// ClearLayerFog uncovers a floor and forgets its shapes.
//
// IT TURNS THE FOG OFF AS WELL AS EMPTYING IT, because those are one thing to
// the person pressing it: "the players can see this floor". Emptying alone would
// leave a prefilled floor covered by nothing but its own flag, which is the same
// screen with a worse explanation.
//
// THE PREFILL IS LEFT WHERE IT IS. It means nothing while fog is off, and the
// first shape drawn on the floor afterwards sets it from that shape's own mode.
// See room.FogAdd.
func (a *App) ClearLayerFog(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the fog", func(layer ulid.ULID) []room.Command {
		return []room.Command{
			&room.FogClear{Layer: layer},
			&room.FogSetEnabled{Layer: layer, Enabled: false},
		}
	})
}
