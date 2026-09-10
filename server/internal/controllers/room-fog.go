package controllers

import (
	"net/http"

	"github.com/oklog/ulid/v2"

	"tabletopper/internal/htmx"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
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
	a.fogCommands(w, r, "fill the fog", func(layer ulid.ULID) []room.Command {
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
	a.fogCommands(w, r, "clear the fog", func(layer ulid.ULID) []room.Command {
		return []room.Command{
			&room.FogClear{Layer: layer},
			&room.FogSetEnabled{Layer: layer, Enabled: false},
		}
	})
}

// fogCommands is layerCommand's shape for a route that is more than one command:
// establish the asker and the room, read the layer out of the FORM, dispatch the
// commands in order, and answer 204 or the first refusal.
//
// A REFUSAL STOPS THE REST. Every command here is refused for the same two
// reasons -- not the GM, or no such floor -- so a second one after a refusal
// would be refused identically and would put a second alert modal behind the
// first.
func (a *App) fogCommands(w http.ResponseWriter, r *http.Request, action string, build func(layer ulid.ULID) []room.Command) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if a.Hub == nil {
		htmx.NotFound(w, "room")

		return
	}

	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")

		return
	}

	// AN ABSENT LAYER IS THE ACTIVE ONE, which is what makes these two items
	// work in a browser where the room bundle never ran -- no WebGL2, a script
	// that threw, a page still loading. The viewed floor defaults to the active
	// floor anyway, so the fallback is the same answer the client would have
	// filled in, and the alternative is a menu item that silently does nothing.
	//
	// A layer that is PRESENT and is not an id is a different thing: a request
	// this server did not write. That is a 404 with nothing in it rather than a
	// message, because the only thing that produces one is somebody posting by
	// hand. Whether the floor exists is the core's question and it answers it
	// into the alert modal.
	var layer ulid.ULID
	if raw := r.FormValue("layer"); raw != "" {
		layer, err = ulid.Parse(raw)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)

			return
		}
	} else {
		view, ok := a.Hub.Table(ctx, row.ID)
		if !ok {
			htmx.NotFound(w, "room")

			return
		}

		layer = view.Table.ActiveLayer
	}

	who := room.Actor{ID: sess.UserID, Role: role}

	for _, cmd := range build(layer) {
		if err := a.Hub.Dispatch(ctx, row.ID, who, cmd); err != nil {
			a.rejectCommand(w, action, err)

			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
