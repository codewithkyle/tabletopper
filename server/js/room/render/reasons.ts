export const AGAIN_STAGES = 1;
export const AGAIN_UPLOADS = 2;
export const AGAIN_DRAGGING = 4;
export const AGAIN_FADING = 8;
export const AGAIN_SWEEPING = 16;
export const AGAIN_TRAVELLING = 32;
const NAMES: readonly (readonly [number, string])[] = [
	[AGAIN_STAGES, "stages"],
	[AGAIN_UPLOADS, "uploads"],
	[AGAIN_DRAGGING, "dragging"],
	[AGAIN_FADING, "fading"],
	[AGAIN_SWEEPING, "sweeping"],
	[AGAIN_TRAVELLING, "travelling"],
];
export function reasonNames(mask: number): string[] {
	const out: string[] = [];
	for (const [bit, name] of NAMES) {
		if ((mask & bit) !== 0) {
			out.push(name);
		}
	}
	return out;
}
export function bit(on: boolean, flag: number): number {
	return on ? flag : 0;
}
