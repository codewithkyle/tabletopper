export interface ContextLoss {
	lost(): boolean;
	stop(): void;
}
export function watchContextLoss(
	mount: HTMLElement, canvas: HTMLCanvasElement, restore: () => void,
): ContextLoss {
	const restoring = mount.querySelector("[data-tabletop-restoring]");
	let down = false;
	function show(on: boolean): void {
		if (restoring instanceof HTMLElement) {
			restoring.hidden = !on;
		}
	}
	function onLost(e: Event): void {
		e.preventDefault();
		down = true;
		show(true);
	}
	function onRestored(): void {
		restore();
		down = false;
		show(false);
	}
	canvas.addEventListener("webglcontextlost", onLost);
	canvas.addEventListener("webglcontextrestored", onRestored);
	return {
		lost: () => down,
		stop() {
			canvas.removeEventListener("webglcontextlost", onLost);
			canvas.removeEventListener("webglcontextrestored", onRestored);
		},
	};
}
