// The one DOM element on the table, and the reason it is one.
//
// A LABEL PER PAWN WOULD LAG AT A FEW HUNDRED. Every element would need a
// transform written per frame while the camera moves, which is hundreds of
// style writes inside the frame loop, and z-ordering them under fog would be a
// second problem with no good answer. So the sprite is drawn and hit-tested in
// canvas and exactly ONE element follows the hovered or selected pawn.
//
// ITS TEXT IS SET ON A CHANGE AND ITS POSITION ON EVERY FRAME. Those are
// different rates: what it says changes when the selection or the pawn does, a
// few times a minute; where it is changes whenever the camera does. Writing the
// text per frame would be a string comparison per field per frame for nothing,
// and writing the transform on a change would leave it behind during a pan.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, which server/js cannot do: it is not a
// Tailwind source, so a class named here would never be emitted. Everything
// this needs is rendered in room-overlay.templ -- including the eight coloured
// condition dots, which live in a <template> and are cloned.

import type { Pawn, Role } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";

export interface OverlayDeps {
	// focus is the one pawn the overlay is about, or null when several or none
	// are chosen.
	focus: () => Pawn | null;

	// selected is how many, which decides which of the two shapes is shown.
	selected: () => string[];

	// bounds is the box to sit above, in map pixels.
	bounds: () => Rect | null;

	// project turns a map point into a CSS pixel offset inside the table.
	project: (x: number, y: number, out: { x: number; y: number }) => { x: number; y: number };

	// layers is the room's floors, for the GM's Move to floor control.
	layers: () => { id: string; name: string }[];

	roomID: string;
	role: Role;
	user: string;
}

export interface Overlay {
	// refresh rewrites the contents. It is called when the selection, the hover
	// or the pawn under either of them changes.
	refresh(): void;

	// place writes the transform. It is called once per frame, and only while
	// there is something to place.
	place(): void;

	stop(): void;
}

// LIFT is how far above the pawn's box the overlay sits, in CSS pixels, so it
// does not cover the thing it is describing.
const LIFT = 8;

export function mountOverlay(mount: HTMLElement, deps: OverlayDeps): Overlay | null {
	const found = mount.querySelector("[data-pawn-overlay]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}

	// The re-declaration is what carries the narrowing above into the closures
	// below: TypeScript does not keep a control-flow narrowing across a function
	// boundary, and every method here is one.
	const root: HTMLElement = found;

	const one = must(root, "[data-overlay-one]");
	const many = must(root, "[data-overlay-many]");
	const name = must(root, "[data-overlay-name]");
	const hp = must(root, "[data-overlay-hp]");
	const ac = must(root, "[data-overlay-ac]");
	const conditions = must(root, "[data-overlay-conditions]");
	const count = root.querySelector("[data-overlay-count]");
	const details = must(root, "[data-overlay-details]");
	const statBlock = must(root, "[data-overlay-stat-block]");
	const edit = must(root, "[data-overlay-edit]");

	// The three GM-only controls are absent from a player's page entirely, so
	// these are nulls rather than hidden elements.
	const remove = root.querySelector("[data-overlay-remove]");
	const layerSelect = root.querySelector("[data-overlay-layer]");
	const dots = root.querySelector("[data-overlay-dots]");

	const at = { x: 0, y: 0 };
	let showing = false;

	// lastLayers is what the floor select was last filled with, so a select
	// somebody has open is not rebuilt underneath them on every refresh.
	let lastLayers = "";

	function refresh(): void {
		const chosen = deps.selected();
		const pawn = deps.focus();

		if (chosen.length > 1) {
			showMany(chosen);
		} else if (pawn) {
			showOne(pawn);
		} else {
			hide();
		}
	}

	function hide(): void {
		if (!showing) {
			return;
		}

		showing = false;
		root.hidden = true;
	}

	function showOne(pawn: Pawn): void {
		showing = true;
		root.hidden = false;
		one.hidden = false;
		many.hidden = true;

		name.textContent = pawn.name;

		// HIT POINTS AS THE VIEWER MAY SEE THEM, and there is nothing to decide
		// here: the projection already did it. A player looking at a monster in
		// a band room was sent a band and no numbers, so what this prints is
		// whichever of the two arrived.
		hp.textContent = pawn.hp !== null
			? `${pawn.hp}${pawn.maxHp !== null ? ` / ${pawn.maxHp}` : ""} HP`
			: pawn.hpBand
				? bandWord(pawn.hpBand)
				: "";

		ac.textContent = pawn.ac !== null ? `AC ${pawn.ac}` : "";

		fillConditions(pawn);

		const window = `pawn:${pawn.id}`;
		details.dataset.window = window;
		details.dataset.windowUrl = `/fragment/room/pawn?room=${deps.roomID}&pawn=${pawn.id}`;
		details.dataset.windowTitle = pawn.name;

		// THE STAT BLOCK IS THE GM'S. A player's projected pawn never carries a
		// monster id -- the server only sets it on the GM's copy -- so this is
		// the projection deciding rather than a flag being trusted.
		if (pawn.monsterId) {
			statBlock.hidden = false;
			statBlock.dataset.window = `monster:${pawn.monsterId}`;
			statBlock.dataset.windowUrl = `/fragment/room/stat-block?room=${deps.roomID}&pawn=${pawn.id}`;
			statBlock.dataset.windowTitle = pawn.name;
		} else {
			statBlock.hidden = true;
		}

		const mayEdit = deps.role === "gm" || (pawn.ownerId !== null && pawn.ownerId === deps.user);
		edit.hidden = !mayEdit;
		if (mayEdit) {
			edit.dataset.modalOpen = `/fragment/room/pawn/edit?room=${deps.roomID}&pawn=${pawn.id}`;
		}
	}

	function showMany(chosen: string[]): void {
		showing = true;
		root.hidden = false;
		one.hidden = true;
		many.hidden = false;

		if (count) {
			count.textContent = `${chosen.length} pawns selected`;
		}

		// hx-vals IS SET AS AN ATTRIBUTE AND READ AT REQUEST TIME, which is what
		// lets one button carry a selection that changes under it. The ids go as
		// one comma-separated value because that is what htmx does with an
		// array -- it sets rather than appends -- and the route splits them.
		const vals = JSON.stringify({ ids: chosen.join(",") });
		remove?.setAttribute("hx-vals", vals);
		layerSelect?.setAttribute("hx-vals", vals);

		fillLayers();
	}

	function fillConditions(pawn: Pawn): void {
		conditions.replaceChildren();
		if (!(dots instanceof HTMLTemplateElement)) {
			return;
		}

		for (const condition of pawn.conditions) {
			const dot = dots.content.querySelector(`[data-dot="${condition.color}"]`);
			if (!dot) {
				continue;
			}

			const copy = dot.cloneNode(true);
			if (copy instanceof HTMLElement) {
				// The name is the title rather than text beside it: sixteen
				// chips would be wider than the table, and the dot's colour is
				// what a GM reads at a glance anyway.
				copy.title = condition.name;
				conditions.append(copy);
			}
		}
	}

	function fillLayers(): void {
		if (!(layerSelect instanceof HTMLSelectElement)) {
			return;
		}

		const layers = deps.layers();
		const key = layers.map((layer) => `${layer.id}:${layer.name}`).join("|");
		if (key === lastLayers) {
			return;
		}
		lastLayers = key;

		layerSelect.replaceChildren();

		const prompt = document.createElement("option");
		prompt.value = "";
		prompt.textContent = "Move to floor...";
		layerSelect.append(prompt);

		for (const layer of layers) {
			const option = document.createElement("option");
			option.value = layer.id;
			option.textContent = layer.name;
			layerSelect.append(option);
		}
	}

	function place(): void {
		if (!showing) {
			return;
		}

		const box = deps.bounds();
		if (!box) {
			hide();

			return;
		}

		// ONE TRANSFORM WRITE AND NOTHING ELSE, which is the whole performance
		// story of this element: no layout is read, no class is toggled, and
		// the compositor moves it without the page being laid out again.
		deps.project((box.x1 + box.x2) / 2, box.y1, at);
		root.style.transform = `translate(${Math.round(at.x)}px, ${Math.round(at.y)}px) translate(-50%, calc(-100% - ${LIFT}px))`;
	}

	return {
		refresh,
		place,
		stop() {
			hide();
		},
	};
}

function bandWord(band: string): string {
	switch (band) {
		case "healthy":
			return "Healthy";
		case "bloodied":
			return "Bloodied";
		case "critical":
			return "Critical";
		case "dead":
			return "Dead";
	}

	return "";
}

function must(root: HTMLElement, selector: string): HTMLElement {
	const found = root.querySelector(selector);
	if (!(found instanceof HTMLElement)) {
		throw new Error(`the overlay is missing ${selector}`);
	}

	return found;
}
