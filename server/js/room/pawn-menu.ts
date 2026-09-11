import type { Named } from "./pawn-window.ts";
import type { Point } from "./render/camera.ts";
import { nextZ } from "./window.ts";
export interface Floor {
	id: string;
	name: string;
}
export interface Target extends Named {
	layerId: string;
}
export interface PawnMenuDeps {
	layers: () => Floor[];
	details: (pawn: Named) => void;
}
export interface PawnMenu {
	open(pawn: Target, screen: Point): void;
	close(): void;
	stop(): void;
}
const EDGE = 8;
export function mountPawnMenu(mount: HTMLElement, deps: PawnMenuDeps): PawnMenu | null {
	const found = mount.querySelector("[data-pawn-menu]");
	if (!(found instanceof HTMLElement)) {
		return null;
	}
	const root: HTMLElement = found;
	const heading = root.querySelector("[data-pawn-menu-name]");
	const floors = root.querySelector("[data-pawn-menu-floors]");
	const list = root.querySelector("[data-pawn-menu-layers]");
	const removeButton = root.querySelector("[data-pawn-menu-remove]");
	const mover = mount.querySelector("[data-pawn-menu-move]");
	const turner = mount.querySelector("[data-pawn-menu-add-turn]");
	const template = mount.querySelector("[data-pawn-menu-template]");
	const shell = template instanceof HTMLTemplateElement ? template : null;
	let target: Target | null = null;
	const origin: Point = { x: 0, y: 0 };
	function open(pawn: Target, screen: Point): void {
		target = pawn;
		if (heading) {
			heading.textContent = pawn.name;
		}
		removeButton?.setAttribute("hx-vals", vals(pawn.id));
		removeButton?.setAttribute(
			"hx-confirm",
			`Remove ${pawn.name} from the table. This cannot be undone.`,
		);
		fillFloors(pawn);
		if (floors instanceof HTMLDetailsElement) {
			floors.open = false;
		}
		root.hidden = false;
		root.style.zIndex = String(nextZ());
		origin.x = screen.x;
		origin.y = screen.y;
		place();
	}
	function close(): void {
		if (root.hidden) {
			return;
		}
		target = null;
		root.hidden = true;
	}
	function place(): void {
		const width = root.offsetWidth;
		const height = root.offsetHeight;
		const right = origin.x + width + EDGE > mount.clientWidth;
		const below = origin.y + height + EDGE > mount.clientHeight;
		const x = clamp(right ? origin.x - width : origin.x, mount.clientWidth - width - EDGE);
		const y = clamp(below ? origin.y - height : origin.y, mount.clientHeight - height - EDGE);
		root.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
	}
	function onToggle(): void {
		if (!root.hidden) {
			place();
		}
	}
	function fillFloors(pawn: Target): void {
		if (!(list instanceof HTMLElement) || !shell) {
			return;
		}
		list.replaceChildren();
		for (const layer of deps.layers()) {
			const row = shell.content.cloneNode(true) as DocumentFragment;
			const button = row.querySelector("[data-pawn-menu-layer]");
			if (!(button instanceof HTMLElement)) {
				continue;
			}
			const name = row.querySelector("[data-pawn-menu-layer-name]");
			const here = row.querySelector("[data-pawn-menu-here]");
			button.dataset.pawnMenuLayer = layer.id;
			if (name) {
				name.textContent = layer.name;
			}
			if (here instanceof HTMLElement) {
				here.hidden = layer.id !== pawn.layerId;
			}
			list.append(row);
		}
	}
	function moveTo(layer: string): void {
		const pawn = target;
		close();
		if (!pawn || layer === "" || layer === pawn.layerId || !(mover instanceof HTMLElement)) {
			return;
		}
		mover.setAttribute("hx-vals", JSON.stringify({ ids: pawn.id, layer }));
		mover.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	}
	function addTurn(): void {
		const pawn = target;
		close();
		if (!pawn || !(turner instanceof HTMLElement)) {
			return;
		}
		turner.setAttribute("hx-vals", JSON.stringify({ pawn: pawn.id }));
		turner.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	}
	function vals(id: string): string {
		return JSON.stringify({ ids: id });
	}
	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}
		if (e.target.closest("[data-pawn-menu-details]")) {
			const pawn = target;
			close();
			if (pawn) {
				deps.details(pawn);
			}
			return;
		}
		if (e.target.closest("[data-pawn-menu-turn]")) {
			addTurn();
			return;
		}
		const floor = e.target.closest("[data-pawn-menu-layer]");
		if (floor instanceof HTMLElement) {
			moveTo(floor.dataset.pawnMenuLayer ?? "");
			return;
		}
		if (e.target.closest("[data-pawn-menu-remove]")) {
			close();
		}
	}
	function onPointerDown(e: Event): void {
		if (root.hidden || (e.target instanceof Node && root.contains(e.target))) {
			return;
		}
		close();
	}
	function onWheel(): void {
		close();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			close();
		}
	}
	root.addEventListener("click", onClick);
	floors?.addEventListener("toggle", onToggle);
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("wheel", onWheel);
	document.addEventListener("keydown", onKeyDown);
	return {
		open,
		close,
		stop() {
			close();
			root.removeEventListener("click", onClick);
			floors?.removeEventListener("toggle", onToggle);
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("wheel", onWheel);
			document.removeEventListener("keydown", onKeyDown);
		},
	};
}
function clamp(value: number, high: number): number {
	return Math.max(EDGE, Math.min(value, Math.max(EDGE, high)));
}
