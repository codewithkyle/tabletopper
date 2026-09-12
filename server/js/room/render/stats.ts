import type { LoaderStats } from "../gl/loader.ts";
import type { StageTiming } from "./stages/list.ts";
import { noLoaderStats } from "../gl/loader.ts";
export interface TextureStats {
	resident: number;
	capacity: number;
	evictions: number;
	loader: LoaderStats;
}
export function noTextureStats(): TextureStats {
	return { resident: 0, capacity: 0, evictions: 0, loader: noLoaderStats() };
}
export interface RenderStats {
	drawn: number;
	last: number;
	average: number;
	p95: number;
	samples: number;
	again: number;
	rebuilds: number;
	calls: number;
	instances: number;
	painted: number;
	sprites: TextureStats;
	tiles: TextureStats;
	zoom: number;
	worldPerCssPixel: number;
	x: number;
	y: number;
	level: number;
	visible: number;
	width: number;
	height: number;
	deviceWidth: number;
	deviceHeight: number;
	dpr: number;
	losses: number;
	gpu: number;
	gpuAvailable: boolean;
	timing: boolean;
	stages: readonly StageTiming[];
}
