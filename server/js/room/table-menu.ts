import type { Grid } from "./protocol.ts";
import type { Point } from "./model/types.ts";
import { cellAt, cellCentre } from "./model/grid.ts";
import { nextZ } from "./window.ts";
export interface MarkedCell {
	x: number;
	y: number;
	size: number;
	centreX: number;
	centreY: number;
}
export interface TableMenuDeps {
	grid: () => Grid;
	viewed: () => string;
	invalidate: () => void;
}
export interface TableMenu {
	open(map: Point, screen: Point): boolean;
	close(): void;
	marked(): MarkedCell | null;
	stop(): void;
}
const FALLBACK_REACH = 64;
export function cellUnder(grid: Grid, map: Point): MarkedCell {
	const [cx, cy] = cellAt(grid, map.x, map.y);
	const [centreX, centreY] = cellCentre(grid, cx, cy);
	const size = Math.max(1, grid.cellSize);
	return { x: centreX - size / 2, y: centreY - size / 2, size, centreX, centreY };
}
export function mountTableMenu(mount: HTMLElement, deps: TableMenuDeps): TableMenu {
	let cell: MarkedCell | null = null;
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
		const found = cellUnder(deps.grid(), map);
		const party = root.querySelector("[data-table-menu-party]");
		if (party instanceof HTMLElement) {
			party.setAttribute(
				"hx-vals",
				JSON.stringify({ layer: deps.viewed(), x: found.centreX, y: found.centreY }),
			);
		}
		cell = found;
		root.hidden = false;
		root.style.zIndex = String(nextZ());
		place(root, screen);
		deps.invalidate();
		return true;
	}
	function close(): void {
		const root = wheel();
		if (cell === null && (root === null || root.hidden)) {
			return;
		}
		cell = null;
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
	function onClick(e: Event): void {
		const root = wheel();
		if (!root || root.hidden || !(e.target instanceof Node) || !root.contains(e.target)) {
			return;
		}
		close();
	}
	function onKeyDown(e: KeyboardEvent): void {
		if (e.key === "Escape") {
			close();
		}
	}
	function onSwap(e: Event): void {
		const shell = host();
		if (!shell || !(e.target instanceof Node) || !shell.contains(e.target)) {
			return;
		}
		cell = null;
		deps.invalidate();
	}
	document.addEventListener("pointerdown", onPointerDown);
	document.addEventListener("click", onClick);
	document.addEventListener("wheel", close);
	document.addEventListener("keydown", onKeyDown);
	document.addEventListener("htmx:after:swap", onSwap);
	return {
		open,
		close,
		marked: () => cell,
		stop() {
			close();
			document.removeEventListener("pointerdown", onPointerDown);
			document.removeEventListener("click", onClick);
			document.removeEventListener("wheel", close);
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("htmx:after:swap", onSwap);
		},
	};
}
function inside(value: number, reach: number, limit: number): number {
	if (limit <= reach * 2) {
		return Math.round(limit / 2);
	}
	return Math.max(reach, Math.min(value, limit - reach));
}
