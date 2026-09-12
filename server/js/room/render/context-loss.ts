export interface ContextLoss {
	lost(): boolean;
	losses(): number;
	stop(): void;
}
export function watchContextLoss(
	mount: HTMLElement, canvas: HTMLCanvasElement, restore: () => void,
): ContextLoss {
	const restoring = mount.querySelector("[data-tabletop-restoring]");
	let down = false;
	let count = 0;
	function show(on: boolean): void {
		if (restoring instanceof HTMLElement) {
			restoring.hidden = !on;
		}
	}
	function onLost(e: Event): void {
		e.preventDefault();
		down = true;
		count++;
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
		losses: () => count,
		stop() {
			canvas.removeEventListener("webglcontextlost", onLost);
			canvas.removeEventListener("webglcontextrestored", onRestored);
		},
	};
}
