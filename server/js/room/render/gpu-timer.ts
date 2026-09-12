interface TimerExtension {
	TIME_ELAPSED_EXT: number;
	GPU_DISJOINT_EXT: number;
}
const NANOS_PER_MS = 1_000_000;
export interface GpuTimer {
	available(): boolean;
	begin(): void;
	end(): void;
	elapsed(): number;
	dispose(): void;
}
export function newGpuTimer(gl: WebGL2RenderingContext): GpuTimer {
	const ext = gl.getExtension("EXT_disjoint_timer_query_webgl2") as TimerExtension | null;
	let pending: WebGLQuery | null = null;
	let open = false;
	let last = -1;
	function collect(): void {
		if (!ext || !pending) {
			return;
		}
		if (gl.getParameter(ext.GPU_DISJOINT_EXT) === true) {
			gl.deleteQuery(pending);
			pending = null;
			last = -1;
			return;
		}
		if (gl.getQueryParameter(pending, gl.QUERY_RESULT_AVAILABLE) !== true) {
			return;
		}
		last = (gl.getQueryParameter(pending, gl.QUERY_RESULT) as number) / NANOS_PER_MS;
		gl.deleteQuery(pending);
		pending = null;
	}
	return {
		available: () => ext !== null,
		begin() {
			if (!ext) {
				return;
			}
			collect();
			if (pending) {
				return;
			}
			const query = gl.createQuery();
			if (!query) {
				return;
			}
			pending = query;
			open = true;
			gl.beginQuery(ext.TIME_ELAPSED_EXT, query);
		},
		end() {
			if (!ext || !open) {
				return;
			}
			open = false;
			gl.endQuery(ext.TIME_ELAPSED_EXT);
		},
		elapsed: () => last,
		dispose() {
			if (open && ext) {
				gl.endQuery(ext.TIME_ELAPSED_EXT);
				open = false;
			}
			if (pending) {
				gl.deleteQuery(pending);
				pending = null;
			}
		},
	};
}
