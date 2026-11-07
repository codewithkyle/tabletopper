export const LONG_PRESS_MS = 500;
export const TOUCH_SLOP = 10;
export interface LongPress {
	touching(): boolean;
	stop(): void;
}
export function moved(fromX: number, fromY: number, toX: number, toY: number): boolean {
	return Math.hypot(toX - fromX, toY - fromY) >= TOUCH_SLOP;
}
export function onLongPress(
	on: HTMLElement,
	fire: (target: Element, clientX: number, clientY: number) => boolean,
): LongPress {
	const fingers = new Set<number>();
	let timer = 0;
	let finger: number | null = null;
	let fromX = 0;
	let fromY = 0;
	let swallow = false;
	function clear(): void {
		if (timer !== 0) {
			window.clearTimeout(timer);
			timer = 0;
		}
		finger = null;
	}
	function onPointerDown(e: PointerEvent): void {
		swallow = false;
		if (e.pointerType !== "touch") {
			return;
		}
		fingers.add(e.pointerId);
		clear();
		if (fingers.size > 1 || !(e.target instanceof Element)) {
			return;
		}
		const target = e.target;
		finger = e.pointerId;
		fromX = e.clientX;
		fromY = e.clientY;
		timer = window.setTimeout(() => {
			timer = 0;
			finger = null;
			swallow = fire(target, fromX, fromY);
		}, LONG_PRESS_MS);
	}
	function onPointerMove(e: PointerEvent): void {
		if (e.pointerId === finger && moved(fromX, fromY, e.clientX, e.clientY)) {
			clear();
		}
	}
	function onPointerEnd(e: PointerEvent): void {
		fingers.delete(e.pointerId);
		if (e.pointerId === finger) {
			clear();
		}
	}
	function onClick(e: Event): void {
		if (!swallow) {
			return;
		}
		swallow = false;
		e.preventDefault();
		e.stopPropagation();
	}
	on.addEventListener("pointerdown", onPointerDown);
	on.addEventListener("pointermove", onPointerMove);
	on.addEventListener("pointerup", onPointerEnd);
	on.addEventListener("pointercancel", onPointerEnd);
	on.addEventListener("click", onClick, true);
	return {
		touching: () => fingers.size > 0,
		stop() {
			clear();
			fingers.clear();
			on.removeEventListener("pointerdown", onPointerDown);
			on.removeEventListener("pointermove", onPointerMove);
			on.removeEventListener("pointerup", onPointerEnd);
			on.removeEventListener("pointercancel", onPointerEnd);
			on.removeEventListener("click", onClick, true);
		},
	};
}
