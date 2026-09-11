import type { Attribute, QuadBatch } from "../gl/quads.ts";
import type { Rgb } from "../model/types.ts";
import { radians } from "../model/shape.ts";
export const SHAPED_QUAD: readonly Attribute[] = [{ size: 4 }, { size: 4 }, { size: 4 }, { size: 2 }];
export function writeShaped(
	batch: QuadBatch,
	x: number, y: number, halfW: number, halfH: number,
	color: Rgb, alpha: number,
	s0: number, s1: number, s2: number, s3: number,
	rotation: number,
): void {
	const at = batch.cursor();
	const data = batch.data;
	data[at] = x;
	data[at + 1] = y;
	data[at + 2] = halfW;
	data[at + 3] = halfH;
	data[at + 4] = color[0];
	data[at + 5] = color[1];
	data[at + 6] = color[2];
	data[at + 7] = alpha;
	data[at + 8] = s0;
	data[at + 9] = s1;
	data[at + 10] = s2;
	data[at + 11] = s3;
	const angle = radians(rotation);
	data[at + 12] = rotation === 0 ? 1 : Math.cos(angle);
	data[at + 13] = rotation === 0 ? 0 : Math.sin(angle);
}
