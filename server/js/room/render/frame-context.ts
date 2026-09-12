import type { Camera, Viewport } from "./camera.ts";
import { clipMatrix, inverseClipMatrix, worldPerCssPixel, worldPerDevicePixel } from "./camera.ts";
import type { Layer, Role, State } from "../protocol.ts";
import type { Overlay } from "../model/overlay.ts";
import type { Painted } from "./layers.ts";
import type { Resources } from "./resources.ts";
import type { Rgb } from "../model/types.ts";
export interface FrameContext {
	gl: WebGL2RenderingContext;
	now: number;
	camera: Camera;
	viewport: Viewport;
	deviceWidth: number;
	deviceHeight: number;
	dpr: number;
	scale: number;
	worldPerCssPixel: number;
	worldPerDevicePixel: number;
	clip: Float32Array;
	clipInverse: Float32Array;
	clear: Rgb;
	role: Role;
	user: string;
	state: State;
	viewed: Layer | null;
	viewedID: string;
	cell: number;
	painted: readonly Painted[];
	rebuild: boolean;
	overlay: Overlay;
	resources: Resources;
}
export function newFrame(
	gl: WebGL2RenderingContext,
	camera: Camera,
	viewport: Viewport,
	role: Role,
	user: string,
	state: State,
	overlay: Overlay,
	resources: Resources,
): FrameContext {
	return {
		gl,
		now: 0,
		camera,
		viewport,
		deviceWidth: 1,
		deviceHeight: 1,
		dpr: 1,
		scale: 1,
		worldPerCssPixel: 1,
		worldPerDevicePixel: 1,
		clip: new Float32Array(9),
		clipInverse: new Float32Array(9),
		clear: [0, 0, 0],
		role,
		user,
		state,
		viewed: null,
		viewedID: "",
		cell: 1,
		painted: [],
		rebuild: true,
		overlay,
		resources,
	};
}
export function sizeFrame(frame: FrameContext, now: number, deviceWidth: number, deviceHeight: number, dpr: number): void {
	const camera = frame.camera;
	frame.now = now;
	frame.deviceWidth = deviceWidth;
	frame.deviceHeight = deviceHeight;
	frame.dpr = dpr;
	frame.scale = camera.zoom * dpr;
	frame.worldPerCssPixel = worldPerCssPixel(camera);
	frame.worldPerDevicePixel = worldPerDevicePixel(camera, dpr);
	clipMatrix(camera, deviceWidth, deviceHeight, dpr, frame.clip);
	inverseClipMatrix(camera, deviceWidth, deviceHeight, dpr, frame.clipInverse);
}
