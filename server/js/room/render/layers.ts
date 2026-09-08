// WHICH FLOOR THIS BROWSER IS LOOKING AT, and the crossfade between two of
// them.
//
// THE ACTIVE LAYER IS THE ROOM'S AND THE VIEWED LAYER IS THE VIEWER'S. Players
// always see the active one -- it is the only one whose pawns reach them at
// all, because the projection in internal/room removes the rest before the
// event is encoded. The GM may look at another to prepare it, which is a
// purely local choice: nothing about it is sent, nothing about it is stored,
// and nobody else can tell.
//
// AND IT FOLLOWS THE ACTIVE LAYER WHENEVER THAT MOVES. A GM who has wandered
// off to the cellar and then makes the first floor active means to be looking
// at the first floor; leaving them on the cellar would put them one command
// behind their own table. So the override is cleared by the active layer
// changing rather than by anything the GM does, and the only way back to a
// different view is to choose one again.
//
// THE OUTGOING MAP IS DRAWN OPAQUE AND THE INCOMING ONE FADES IN OVER IT, which
// is not the "one falls while the other rises" the first sketch of this called
// for. Complementary alphas are wrong over a background: clearing to the table
// colour, then drawing the old map at 1-t and the new at t, leaves
// t*new + (1-t)^2*old + t(1-t)*background -- a quarter of the table colour
// showing through at the midpoint, so the transition dips through the empty
// desk and back. Painting the old one solid and dissolving the new one over it
// is exactly lerp(old, new, t) and dips through nothing.
//
// THE CROSSFADE IS 250 MILLISECONDS AND IT IS BETWEEN MAPS, NOT LAYERS.
// Switching to another floor that happens to use the same map is not a change
// anybody should see, and re-picking the map on the floor you are looking at
// is. What is compared is the asset and its tiling generation, which is
// precisely what decides the tile URLs.

import type { Layer, MapRef, Table } from "../protocol.ts";

// CROSSFADE_MS is long enough to read as a transition and short enough that a
// GM flipping between two floors to compare them is not waiting on it.
export const CROSSFADE_MS = 250;

// Painted is one map the tile pass draws this frame, and the opacity to draw it
// at. During a crossfade there are two of these, the outgoing one first.
export interface Painted {
	map: MapRef;
	alpha: number;
}

export interface LayerView {
	// update settles the viewed layer and the fade against the table as it is
	// now. It runs once per frame, before anything reads the three below.
	update(table: Table, now: number): void;

	// choose is the GM picking a floor to look at. An id that is not a layer,
	// and any call at all from a player, is ignored.
	choose(id: string): void;

	viewed(): Layer | null;

	// draws is what to paint, outgoing map first and opaque, incoming second at
	// the fade's progress. It returns a live array that is reused every frame:
	// read it and do not keep it.
	draws(): readonly Painted[];

	// fading is what keeps the frame loop alive through the transition.
	fading(): boolean;

	// following is whether the viewed layer is the active one, which is what
	// the bar's badge and its Make active button are shown by.
	following(): boolean;
}

export function newLayerView(isGM: boolean): LayerView {
	// override is the GM's choice, and the empty string means "whatever is
	// active". It is deliberately not initialised to the active layer: a value
	// that starts as a copy has to be kept in step, and an absence does not.
	let override = "";
	let lastActive = "";

	let layer: Layer | null = null;
	let activeID = "";

	let current: MapRef | null = null;
	let previous: MapRef | null = null;
	let fadeFrom = 0;
	let at = 0;

	// The two slots are reused so that a frame costs no allocation. draws()
	// hands back the same array with the same two objects in it every time.
	const outgoing: Painted = { map: emptyMap(), alpha: 0 };
	const incoming: Painted = { map: emptyMap(), alpha: 0 };
	const painted: Painted[] = [];

	function progress(): number {
		if (!previous) {
			return 1;
		}

		const t = (at - fadeFrom) / CROSSFADE_MS;

		return t >= 1 ? 1 : t <= 0 ? 0 : t;
	}

	return {
		update(table, now) {
			at = now;
			activeID = table.activeLayer;

			// THE ACTIVE LAYER MOVING IS WHAT CLEARS THE OVERRIDE, and it is
			// checked before the layer is resolved so that the same frame that
			// learns of the change is already following it.
			if (table.activeLayer !== lastActive) {
				lastActive = table.activeLayer;
				override = "";
			}

			// A GM viewing a floor that has just been deleted falls back to the
			// active one rather than to nothing, which is where the core put
			// them anyway.
			layer = find(table, override) ?? find(table, table.activeLayer);

			const map = layer?.map ?? null;
			if (key(map) !== key(current)) {
				previous = current;
				current = map;

				// A FADE NEEDS TWO THINGS TO CROSS. The first map of the
				// session arrives over an empty table, and fading that in from
				// nothing is a quarter second of blank screen for no reason.
				fadeFrom = previous ? now : now - CROSSFADE_MS;
			}

			// Dropping the outgoing map the moment it is invisible is what
			// lets fading() go false and the frame loop go idle.
			if (previous && progress() >= 1) {
				previous = null;
			}
		},

		choose(id) {
			if (!isGM) {
				return;
			}

			override = id === activeID ? "" : id;
		},

		viewed: () => layer,

		draws() {
			painted.length = 0;

			const t = progress();

			if (previous && t < 1) {
				outgoing.map = previous;
				outgoing.alpha = 1;
				painted.push(outgoing);
			}

			if (current) {
				incoming.map = current;
				incoming.alpha = previous ? t : 1;
				painted.push(incoming);
			}

			return painted;
		},

		fading: () => previous !== null,

		following: () => layer === null || layer.id === activeID,
	};
}

function find(table: Table, id: string): Layer | null {
	if (id === "") {
		return null;
	}

	for (const l of table.layers) {
		if (l.id === id) {
			return l;
		}
	}

	return null;
}

// key is a map's identity for the purpose of "is this the same picture". The
// generation is in it because a re-tiled map is a new pyramid at new URLs, and
// the whole point of the fade is that the tiles underneath are changing.
function key(map: MapRef | null): string {
	return map ? `${map.assetId}:${map.gen}` : "";
}

function emptyMap(): MapRef {
	return { assetId: "", gen: "", width: 0, height: 0, tileSize: 0, maxZoom: 0 };
}
