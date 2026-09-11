






























export interface LayeredMenu {
	refresh(): void;
}

export function mountLayeredMenu(viewed: () => string): LayeredMenu | null {
	const items = Array.from(document.querySelectorAll("[data-room-layered]"));
	if (items.length === 0) {
		return null;
	}

	
	
	
	let last = "";

	function refresh(): void {
		const layer = viewed();
		if (layer === last) {
			return;
		}
		last = layer;

		const vals = layer === "" ? "{}" : JSON.stringify({ layer });
		for (const item of items) {
			item.setAttribute("hx-vals", vals);
		}
	}

	refresh();

	return { refresh };
}
