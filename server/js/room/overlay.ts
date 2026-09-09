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
// IT FOLLOWS THE HOVER AND NEVER THE SELECTION. A pawn somebody has SELECTED is
// a pawn they are about to do something to -- drag it, turn it, resize it --
// and a panel parked over the top of it is in the way of every one of those.
// What a label is for is telling you what you are pointing AT, which is a
// question you stop asking the moment you have picked the thing. The one
// exception is a multiple selection, where the label is not describing a pawn
// at all: it is the count and the three controls that act on the group.
//
// IT CARRIES NO BUTTONS WHILE IT IS DESCRIBING ONE PAWN. Everything about a
// pawn is in its window, which a double click on the table opens and the right
// click's menu offers; a row of buttons on a thing that follows the pointer is
// a row of buttons that moves out from under the hand reaching for it.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, which server/js cannot do: it is not a
// Tailwind source, so a class named here would never be emitted. Everything
// this needs is rendered in room-overlay.templ.

import type { Pawn } from "./protocol.ts";
import type { Rect } from "./render/camera.ts";

export interface OverlayDeps {
	// focus is the one pawn the overlay is describing -- the hovered one -- or
	// null when there is nothing under the pointer worth a label.
	focus: () => Pawn | null;

	// selected is how many, which is what decides between describing one pawn
	// and counting a group.
	selected: () => string[];

	// bounds is the box to sit above, in map pixels, and it is the box of
	// whatever is being shown rather than of whatever is selected.
	bounds: () => Rect | null;

	// project turns a map point into a CSS pixel offset inside the table.
	project: (x: number, y: number, out: { x: number; y: number }) => { x: number; y: number };

	// layers is the room's floors, for the GM's Move to floor control.
	layers: () => { id: string; name: string }[];

	// anyShown is whether ANY of these pawns is currently visible to players,
	// which is the whole of what the group's hide-and-reveal button needs.
	//
	// ANY RATHER THAN ALL, because a mixed selection has to resolve to one of
	// the two words and "Hide" is the one that finishes the job: pressing it
	// leaves nothing on the table the players can see, which is what somebody
	// who marqueed a corridor full of goblins and reached for this meant. All
	// would leave the visible ones visible and read as a button that did
	// nothing.
	//
	// IT IS ASKED OF THE STORE AND NOT REMEMBERED, so a pawn another GM
	// revealed a moment ago is counted -- the socket event that arrived is
	// what refreshes this panel.
	anyShown: (ids: string[]) => boolean;

	// labels is the room's one setting for this panel, and reading it here is
	// the ONLY branch in the client on what a viewer may know.
	//
	// EVERYTHING ELSE IS ALREADY DECIDED BY THE TIME A PAWN ARRIVES. A player
	// in a default room was sent a word and no numbers and a player in a full
	// room was sent both, so showOne prints whatever is on the pawn and needs
	// no idea which room it is in. "None" is the one setting that cannot work
	// that way: it takes the panel away from the GM too, and a GM's copy of a
	// pawn is never projected -- there is nothing missing from it to notice.
	//
	// IT DOES NOT TAKE THE GROUP PANEL AWAY, because that one is not a label.
	// What it holds is a count and the GM's three controls for acting on a
	// selection: a toolbar that happens to follow what is selected, rather than
	// anything a viewer is being told about a pawn.
	labels: () => string;
}

export interface Overlay {
	// refresh rewrites the contents. It is called when the selection, the hover
	// or the pawn under either of them changes.
	refresh(): void;

	// remove presses the Delete key's own button, which is a hidden control on
	// the room page rather than anything in this panel.
	//
	// A CLICK AND NOT A FETCH, and that is the whole reason it is a button at
	// all. Removing pawns is confirmed, the confirmation is the app's confirm
	// modal, and hx-confirm lives on the element that makes the request -- so
	// pressing that element is what gets the dialog. Building the DELETE by
	// hand would skip it, and window.confirm is banned.
	//
	// IT IS HIDDEN AND IT STAYS HIDDEN. A keyboard shortcut needs an element to
	// press; it does not need one anybody can see, and the panel this file
	// draws deliberately has no buttons on it.
	//
	// IT IS THE GM'S BUTTON OR NOTHING. A player's page never renders one, so
	// this is a no-op for them without a role test of its own.
	remove(): void;

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
	const count = root.querySelector("[data-overlay-count]");

	// The GM-only controls are absent from a player's page entirely, so these
	// are nulls rather than hidden elements. The Delete key's button is looked
	// up on the MOUNT rather than on the panel, because it is not part of it.
	const removeButton = root.querySelector("[data-overlay-remove]");
	const shownButton = root.querySelector("[data-overlay-shown]");
	const layerSelect = root.querySelector("[data-overlay-layer]");
	const removeKey = mount.querySelector("[data-pawn-remove]");

	const at = { x: 0, y: 0 };
	let showing = false;

	// lastLayers is what the floor select was last filled with, so a select
	// somebody has open is not rebuilt underneath them on every refresh.
	let lastLayers = "";

	function refresh(): void {
		const chosen = deps.selected();

		// THE KEY'S BUTTON IS ARMED WHATEVER THE PANEL DOES, because the two
		// are not the same question. Delete acts on the SELECTION, and the
		// selection is exactly the case this panel goes away for.
		armRemoveKey(chosen);

		if (chosen.length > 1) {
			showMany(chosen);

			return;
		}

		// A SINGLE SELECTION SHOWS NOTHING AT ALL, which is the point: what a
		// hand does next to one selected pawn is drag it, turn it or resize it,
		// and all three happen where this panel would be sitting. Hovering
		// something else while one thing is selected still labels what is under
		// the pointer, because that is the question a label answers.
		const pawn = deps.focus();
		if (pawn && deps.labels() !== "none") {
			showOne(pawn);

			return;
		}

		hide();
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
		// a default room was sent a word and no numbers, so what this prints is
		// whichever of the two arrived. The same goes for the line below it: a
		// null armour class is one the server withheld, not one nobody set.
		hp.textContent = pawn.hp !== null
			? `${pawn.hp}${pawn.maxHp !== null ? ` / ${pawn.maxHp}` : ""} HP`
			: pawn.hpBand
				? bandWord(pawn.hpBand)
				: "";

		ac.textContent = pawn.ac !== null ? `AC ${pawn.ac}` : "";
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
		// lets one button carry a selection that changes under it.
		const values = vals(chosen);
		removeButton?.setAttribute("hx-vals", values);
		layerSelect?.setAttribute("hx-vals", values);

		armShown(chosen);
		fillLayers();
	}

	// armShown writes both halves of the hide-and-reveal button: what it says,
	// and the state it asks for.
	//
	// THE STATE GOES ON THE REQUEST RATHER THAN BEING WORKED OUT AT THE OTHER
	// END, and the word on the button is the same answer read twice. A route
	// that toggled whatever it found would flip twice when two GMs pressed it
	// at once and land where neither of them meant; this way a press does what
	// the person could read.
	//
	// THE FIELD IS CALLED shown AND NOT visible, which is not a preference: a
	// form field is a candidate the stylesheet's extractor takes off an
	// attribute, and `visible` is a utility class. The pawn's own panel posts
	// the same name for the same switch.
	function armShown(chosen: string[]): void {
		if (!(shownButton instanceof HTMLElement)) {
			return;
		}

		const hide = deps.anyShown(chosen);

		shownButton.textContent = hide ? "Hide" : "Reveal";
		shownButton.setAttribute("hx-vals", JSON.stringify({
			ids: chosen.join(","),
			shown: hide ? "" : "on",
		}));
	}

	// armRemoveKey keeps the hidden button in step with the selection, so the
	// Delete key never removes something that is no longer chosen.
	function armRemoveKey(chosen: string[]): void {
		removeKey?.setAttribute("hx-vals", vals(chosen));
	}

	// vals is the ids in the shape the route reads.
	//
	// ONE COMMA-SEPARATED VALUE AND NOT A REPEATED FIELD, because that is what
	// htmx does with an array -- it SETS each key rather than appending it --
	// and the route splits on the comma. A ULID has none in it.
	function vals(chosen: string[]): string {
		return JSON.stringify({ ids: chosen.join(",") });
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

		remove() {
			if (deps.selected().length === 0) {
				return;
			}

			removeKey?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		},

		stop() {
			hide();
		},
	};
}

// bandWord is the word a band prints as, and it is exported for its test rather
// than for a caller: pages.PawnBandText says the same six in the pawn's window,
// and one goblin described two ways in two places is the failure both sides are
// pinned against.
export function bandWord(band: string): string {
	switch (band) {
		case "healthy":
			return "Healthy";
		case "bruised":
			return "Bruised";
		case "bloody":
			return "Bloody";
		case "veryBloody":
			return "Very bloody";
		case "nearDeath":
			return "Near death";
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
