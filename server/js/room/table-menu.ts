import type { Grid } from "./protocol.ts";
import type { Outgoing } from "./socket.ts";
import type { Point } from "./model/types.ts";
import type { Stamp } from "./model/overlay.ts";
import { cellAt, cellCentre } from "./model/grid.ts";
import { isHex } from "./model/hex.ts";
import { nextZ } from "./window.ts";
export interface MarkedCell {
	x: number;
	y: number;
	size: number;
	centreX: number;
	centreY: number;
	q: number;
	r: number;
}
export interface TableMenuDeps {
	grid: () => Grid;
	viewed: () => string;
	partyStart: () => Point | null;
	scale: () => number;
	invalidate: () => void;
	send: (command: Outgoing) => void;
}
export interface TableMenu {
	open(map: Point, screen: Point): boolean;
	close(): void;
	marked(): MarkedCell | null;
	preview(): Stamp | null;
	stop(): void;
}
interface Picked {
	art: string;
	image: string;
}
const FALLBACK_REACH = 64;
export function cellUnder(grid: Grid, map: Point): MarkedCell {
	const [q, r] = cellAt(grid, map.x, map.y);
	const [centreX, centreY] = cellCentre(grid, q, r);
	const size = Math.max(1, grid.cellSize);
	return { x: centreX - size / 2, y: centreY - size / 2, size, centreX, centreY, q, r };
}
export function mountTableMenu(mount: HTMLElement, deps: TableMenuDeps): TableMenu {
	let cell: MarkedCell | null = null;
	let hovered: Picked | null = null;
	let turn = 0;
	function host(): HTMLElement | null {
		const found = mount.querySelector("[data-table-menu-host]");
		return found instanceof HTMLElement ? found : null;
	}
	function wheel(): HTMLElement | null {
		const found = host()?.querySelector("[data-table-menu]");
		return found instanceof HTMLElement ? found : null;
	}
	function open(map: Point, screen: Point): boolean {
		const root = wheel();
		if (!root) {
			return false;
		}
		const grid = deps.grid();
		const found = cellUnder(grid, map);
		const party = root.querySelector("[data-table-menu-party]");
		if (party instanceof HTMLElement) {
			const layer = deps.viewed();
			const taken = holds(grid, deps.partyStart(), found);
			party.setAttribute(
				"hx-vals",
				JSON.stringify(taken ? { layer } : { layer, x: found.centreX, y: found.centreY }),
			);
			const label = party.getAttribute(taken ? "data-clear-label" : "data-set-label") ?? "";
			party.setAttribute("data-tip", label);
			party.setAttribute("aria-label", label);
		}
		cell = found;
		root.hidden = false;
		root.style.zIndex = String(nextZ());
		place(root, centred(found, map, screen, deps.scale()));
		deps.invalidate();
		return true;
	}
	function close(): void {
		const root = wheel();
		if (cell === null && (root === null || root.hidden)) {
			return;
		}
		cell = null;
		hovered = null;
		if (root) {
			root.hidden = true;
		}
		deps.invalidate();
	}
	function place(root: HTMLElement, screen: Point): void {
		const shell = host();
		const reach = Number.parseInt(root.getAttribute("data-reach") ?? "", 10) || FALLBACK_REACH;
		const x = inside(screen.x, reach, shell?.clientWidth ?? 0);
		const y = inside(screen.y, reach, shell?.clientHeight ?? 0);
		root.style.transform = `translate(${Math.round(x)}px, ${Math.round(y)}px)`;
	}
	function onPointerDown(e: Event): void {
		const root = wheel();
		if (!root || root.hidden) {
			return;
		}
		if (e.target instanceof Node && root.contains(e.target)) {
			return;
		}
		close();
	}
	function step(): number {
		return isHex(deps.grid()) ? 60 : 90;
	}
	function turned(): number {
		const by = step();
		return ((Math.round(turn / by) * by) % 360 + 360) % 360;
	}
	function picked(e: Event): Picked | null {
		if (!(e.target instanceof Element)) {
			return null;
		}
		const art = e.target.closest("[data-table-menu-art]");
		if (art instanceof HTMLElement) {
			return {
				art: art.getAttribute("data-table-menu-art") ?? "",
				image: art.getAttribute("data-image") ?? "",
			};
		}
		if (e.target.closest("[data-table-menu-erase]") !== null) {
			return { art: "", image: "" };
		}
		return null;
	}
	function onClick(e: Event): void {
		const root = wheel();
		if (!root || root.hidden || !(e.target instanceof Node) || !root.contains(e.target)) {
			return;
		}
		const chosen = picked(e);
		const at = cell;
		if (chosen && at) {
			const layer = deps.viewed();
			if (chosen.art === "") {
				deps.send({ type: "tiles.erase", layer, cells: [{ q: at.q, r: at.r }] });
			} else {
				deps.send({
					type: "tiles.stamp",
					layer,
					art: chosen.art,
					rotation: turned(),
					cells: [{ q: at.q, r: at.r }],
				});
			}
		}
		close();
	}
	function onPointerOver(e: Event): void {
		const root = wheel();
		if (!root || root.hidden || !(e.target instanceof Node) || !root.contains(e.target)) {
			return;
		}
		const chosen = picked(e);
		const next = chosen && chosen.art !== "" ? chosen : null;
		if (next?.art === hovered?.art) {
			return;
		}
		hovered = next;
		deps.invalidate();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			close();
			return;
		}
		if (cell === null || e.ctrlKey || e.metaKey || e.altKey) {
			return;
		}
		if (e.key !== "[" && e.key !== "]") {
			return;
		}
		turn = turned() + (e.key === "]" ? step() : -step());
		deps.invalidate();
	}
	function onSwap(e: Event): void {
		const shell = host();
		if (!shell || !(e.target instanceof Node) || !shell.contains(e.target)) {
			return;
		}
		cell = null;
		hovered = null;
		deps.invalidate();
	}
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("click", onClick);
	document.addEventListener("pointerover", onPointerOver);
	document.addEventListener("wheel", close);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:after:swap", onSwap);
	return {
		open,
		close,
		marked: () => cell,
		preview() {
			if (!cell || !hovered) {
				return null;
			}
			return { image: hovered.image, q: cell.q, r: cell.r, rotation: turned() };
		},
		stop() {
			close();
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("click", onClick);
			document.removeEventListener("pointerover", onPointerOver);
			document.removeEventListener("wheel", close);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:after:swap", onSwap);
		},
	};
}
function centred(cell: MarkedCell, map: Point, screen: Point, scale: number): Point {
	const perPixel = scale > 0 ? scale : 1;
	return {
		x: screen.x + (cell.centreX - map.x) / perPixel,
		y: screen.y + (cell.centreY - map.y) / perPixel,
	};
}
function holds(grid: Grid, start: Point | null, cell: MarkedCell): boolean {
	if (!start) {
		return false;
	}
	const at = cellUnder(grid, start);
	return at.centreX === cell.centreX && at.centreY === cell.centreY;
}
function inside(value: number, reach: number, limit: number): number {
	if (limit <= reach * 2) {
		return Math.round(limit / 2);
	}
	return Math.max(reach, Math.min(value, limit - reach));
}
