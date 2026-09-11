import { THEME_CHANGE } from "../../../public/js/events.js";
import type { Rgb } from "../model/types.ts";
export interface Theme {
	readonly color: Rgb;
	stop(): void;
}
export function watchTheme(mount: HTMLElement, changed: () => void): Theme {
	const color: [number, number, number] = [0, 0, 0];
	let probe: CanvasRenderingContext2D | null = null;
	function read(): void {
		probe ??= document.createElement("canvas").getContext("2d", { willReadFrequently: true });
		if (!probe) {
			return;
		}
		probe.fillStyle = "#000000";
		probe.fillStyle = window.getComputedStyle(mount).backgroundColor;
		probe.fillRect(0, 0, 1, 1);
		const pixel = probe.getImageData(0, 0, 1, 1).data;
		color[0] = pixel[0] / 255;
		color[1] = pixel[1] / 255;
		color[2] = pixel[2] / 255;
		changed();
	}
	read();
	window.addEventListener(THEME_CHANGE, read);
	return {
		color,
		stop() {
			window.removeEventListener(THEME_CHANGE, read);
		},
	};
}
