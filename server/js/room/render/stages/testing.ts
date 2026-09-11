export interface Call {
	name: string;
	args: unknown[];
}
export interface RecordingGL {
	gl: WebGL2RenderingContext;
	calls: Call[];
	draws(): string[];
	reset(): void;
}
const CONSTANTS: Record<string, number> = {
	MAX_ARRAY_TEXTURE_LAYERS: 256,
	LINK_STATUS: 1,
	COMPILE_STATUS: 1,
};
export function recordingGL(): RecordingGL {
	const calls: Call[] = [];
	let next = 1;
	const target = {} as Record<string, unknown>;
	const gl = new Proxy(target, {
		get(_, property: string) {
			if (property === Symbol.toPrimitive as unknown as string) {
				return undefined;
			}
			if (/^[A-Z0-9_]+$/.test(property)) {
				return CONSTANTS[property] ?? next++;
			}
			return (...args: unknown[]): unknown => {
				calls.push({ name: property, args });
				if (property.startsWith("create")) {
					return { handle: next++, kind: property };
				}
				if (property === "getProgramParameter" || property === "getShaderParameter") {
					return true;
				}
				if (property === "getUniformLocation") {
					return { uniform: String(args[1]) };
				}
				if (property === "getParameter") {
					return 256;
				}
				if (property === "getProgramInfoLog" || property === "getShaderInfoLog") {
					return "";
				}
				return undefined;
			};
		},
	}) as unknown as WebGL2RenderingContext;
	return {
		gl,
		calls,
		draws() {
			return calls
				.filter((call) => call.name === "drawArrays" || call.name === "drawArraysInstanced")
				.map((call) => call.name);
		},
		reset() {
			calls.length = 0;
		},
	};
}
export interface Fake2D {
	restore(): void;
}
export function fakeDOM(): Fake2D {
	const globals = globalThis as Record<string, unknown>;
	const had = { document: globals.document, window: globals.window };
	const context2d = {
		font: "",
		fillStyle: "",
		textAlign: "",
		textBaseline: "",
		measureText: () => ({ width: 10 }),
		fillRect: () => {},
		fillText: () => {},
		getImageData: () => ({ data: [0, 0, 0, 255] }),
	};
	const canvas = {
		width: 1,
		height: 1,
		getContext: () => context2d,
	};
	globals.document = { createElement: () => ({ ...canvas }) };
	globals.window = {
		addEventListener: () => {},
		removeEventListener: () => {},
		getComputedStyle: () => ({ backgroundColor: "#000000" }),
		devicePixelRatio: 1,
	};
	return {
		restore() {
			globals.document = had.document;
			globals.window = had.window;
		},
	};
}
