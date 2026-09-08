// The bridge from the socket to the DOM panels, and it is four lines of
// mapping because that is the whole of it.
//
// LIVE PANELS UPDATE BY REFETCH. A socket event fires a DOM event on window, an
// element's hx-trigger hears it, and htmx fetches a room fragment that reads
// the in-memory room. Six clients doing one GET per hit-point change is
// nothing, and it keeps rendered markup on HTTP where every other page in this
// app keeps it -- the socket carries JSON and nothing else.
//
// A PANEL THEREFORE NEEDS NO JAVASCRIPT OF ITS OWN. It declares what it listens
// for in its own markup:
//
//     hx-get="/fragment/room/members?room=..." hx-trigger="room:players from:window"
//
// THIS IS ALSO WHY EVERY TABLE MUTATION ANSWERS 204. The layer manager, the
// grid form and the layer name in the bar all listen for room:tabletop, and every
// one of the GM's controls ends in table.updated -- so the window that sent the
// command and the one open in another tab are corrected by the same event, from
// the same source, at the same time. A reply carrying the new markup would
// correct one of them.

import type { Event } from "./protocol.ts";

// panelEvents is which DOM event each family of protocol events raises. A
// family that no panel listens for is simply absent.
const panelEvents: Partial<Record<Event["type"], string>> = {
	"player.joined": "room:players",
	"player.updated": "room:players",
	"player.left": "room:players",
	"initiative.updated": "room:initiative",
	"room.updated": "room:info",
	"table.updated": "room:tabletop",
};

// A snapshot changes everything at once, so it raises everything at once. This
// is the first join and every resync, which is exactly when a panel drawn from
// a stale fetch would be wrong.
const everything = ["room:players", "room:initiative", "room:info", "room:tabletop"];

export function announce(event: Event): void {
	if (event.type === "snapshot") {
		for (const name of everything) {
			window.dispatchEvent(new CustomEvent(name));
		}

		return;
	}

	const name = panelEvents[event.type];
	if (name) {
		window.dispatchEvent(new CustomEvent(name));
	}
}
