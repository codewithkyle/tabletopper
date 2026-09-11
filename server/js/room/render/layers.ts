































import type { Layer, MapRef, Table } from "../protocol.ts";



export const CROSSFADE_MS = 250;



export interface Painted {
	map: MapRef;
	alpha: number;
}

export interface LayerView {
	
	
	update(table: Table, now: number): void;

	
	
	choose(id: string): void;

	viewed(): Layer | null;

	
	
	
	draws(): readonly Painted[];

	
	fading(): boolean;

	
	
	following(): boolean;
}

export function newLayerView(isGM: boolean): LayerView {
	
	
	
	let override = "";
	let lastActive = "";

	let layer: Layer | null = null;
	let activeID = "";

	let current: MapRef | null = null;
	let previous: MapRef | null = null;
	let fadeFrom = 0;
	let at = 0;

	
	
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

			
			
			
			if (table.activeLayer !== lastActive) {
				lastActive = table.activeLayer;
				override = "";
			}

			
			
			
			layer = find(table, override) ?? find(table, table.activeLayer);

			const map = layer?.map ?? null;
			if (key(map) !== key(current)) {
				previous = current;
				current = map;

				
				
				
				fadeFrom = previous ? now : now - CROSSFADE_MS;
			}

			
			
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




function key(map: MapRef | null): string {
	return map ? `${map.assetId}:${map.gen}` : "";
}

function emptyMap(): MapRef {
	return { assetId: "", gen: "", width: 0, height: 0, tileSize: 0, maxZoom: 0 };
}
