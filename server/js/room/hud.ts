import type { Pawn } from "./protocol.ts";
import type { Rect } from "./model/types.ts";
export interface HudDeps {
	focus: () => Pawn | null;
	selected: () => string[];
	bounds: () => Rect | null;
	project: (x: number, y: number, out: { x: number; y: number }) => { x: number; y: number };
	layers: () => { id: string; name: string }[];
	anyShown: (ids: string[]) => boolean;
	labels: () => string;
}
export interface Hud {
	refresh(): void;
	remove(): void;
	place(): void;
	stop(): void;
}
const LIFT = 8;
function removePrompt(count: number): string {
	const what = count === 1 ? "the selected pawn" : `the ${count} selected pawns`;
	return `Remove ${what} from the table. This cannot be undone.`;
}
export function mountHud(mount: HTMLElement, deps: HudDeps): Hud | null {
	const found = mount.querySelector("[data-pawn-overlay]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const root: HTMLElement = found;
	const one = must(root, "[data-overlay-one]");
	const many = must(root, "[data-overlay-many]");
	const name = must(root, "[data-overlay-name]");
	const hp = must(root, "[data-overlay-hp]");
	const ac = must(root, "[data-overlay-ac]");
	const count = root.querySelector("[data-overlay-count]");
	const removeButton = root.querySelector("[data-overlay-remove]");
	const shownButton = root.querySelector("[data-overlay-shown]");
	const layerSelect = root.querySelector("[data-overlay-layer]");
	const removeKey = mount.querySelector("[data-pawn-remove]");
	const at = { x: 0, y: 0 };
	let showing = false;
	let lastLayers = "";
	function refresh(): void {
		const chosen = deps.selected();
		armRemoveKey(chosen);
		if (chosen.length > 1) {
			showMany(chosen);
			return;
		}
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
		hp.textContent = pawn.hpBand !== null
			? bandWord(pawn.hpBand)
			: pawn.hp !== null
				? `${pawn.hp}${pawn.maxHp !== null ? ` / ${pawn.maxHp}` : ""} HP`
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
		const values = vals(chosen);
		removeButton?.setAttribute("hx-vals", values);
		layerSelect?.setAttribute("hx-vals", values);
		armShown(chosen);
		fillLayers();
	}
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
	function armRemoveKey(chosen: string[]): void {
		removeKey?.setAttribute("hx-vals", vals(chosen));
		removeKey?.setAttribute("hx-confirm", removePrompt(chosen.length));
	}
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
		throw new Error(`the pawn hud is missing ${selector}`);
	}
	return found;
}
