import type { Tool } from "../render/input.ts";
export function createPan(): Tool {
	return {
		press: () => false,
		drag() {},
		release() {},
		cancel() {},
		secondary: () => false,
		hover() {},
		key: () => false,
		abandon: () => false,
		active: () => false,
		contribute() {},
	};
}
