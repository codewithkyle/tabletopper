import type { Event, State } from "./protocol.ts";
import type { Renderer } from "./render/renderer.ts";
import type { Socket, Status } from "./socket.ts";

const historyLimit = 20;

const STRESS_PAWNS = 500;

export function wireDebug(root: HTMLElement, socket: Socket, state: State, renderer: Renderer | null): {
	status(status: Status, detail: string): void;
	event(event: Event): void;
} {
	const connection = root.querySelector("[data-debug-connection]");
	const sequence = root.querySelector("[data-debug-seq]");
	const build = root.querySelector("[data-debug-version]");
	const layer = root.querySelector("[data-debug-layer]");
	const players = root.querySelector("[data-debug-players]");
	const events = root.querySelector("[data-debug-events]");
	const form = root.querySelector("[data-debug-form]");
	const input = root.querySelector("[data-debug-input]");

	const benchmark = root.querySelector("[data-debug-benchmark]");
	const timing = root.querySelector("[data-debug-timing]");

	let stressed = 0;

	if (benchmark instanceof HTMLButtonElement && renderer) {
		benchmark.addEventListener("click", () => {
			benchmark.disabled = true;
			if (timing) {
				timing.textContent = "sweeping...";
			}

			renderer.benchmark((result) => {
				benchmark.disabled = false;
				if (timing) {
					timing.textContent =
						`avg ${result.average.toFixed(2)}ms  p95 ${result.p95.toFixed(2)}ms  ` +
						`${result.frames} frames  ${result.tiles} tiles`;
				}
			});
		});
	}

	const stress = root.querySelector("[data-debug-stress]");
	if (stress instanceof HTMLButtonElement && renderer) {
		stress.addEventListener("click", () => {
			const added = renderer.stress(stressed > 0 ? 0 : STRESS_PAWNS);
			stressed = added;
			stress.textContent = added > 0 ? `Stress (${added})` : "Stress";
		});
	}

	if (form instanceof HTMLFormElement && input instanceof HTMLTextAreaElement) {
		form.addEventListener("submit", (e) => {
			e.preventDefault();

			const text = input.value.trim();
			if (text === "") {
				return;
			}
			if (socket.raw(text)) {
				input.value = "";
			}
		});
	}

	function refresh(): void {
		if (sequence) {
			sequence.textContent = String(socket.sequence());
		}
		if (build) {
			build.textContent = socket.build();
		}
		if (layer) {
			layer.textContent = state.table.activeLayer;
		}
		if (players) {
			players.replaceChildren(
				...state.players.map((player) => {
					const line = document.createElement("li");
					line.textContent = `${player.name} (${player.role})${player.connected ? "" : " - away"}`;

					return line;
				}),
			);
		}
	}

	return {
		status(status, detail) {
			if (connection) {
				connection.textContent = detail === "" ? status : `${status} - ${detail}`;
			}
			refresh();
		},

		event(event) {
			if (events) {
				const line = document.createElement("li");
				line.textContent = JSON.stringify(event);
				events.prepend(line);

				while (events.childElementCount > historyLimit) {
					events.lastElementChild?.remove();
				}
			}
			refresh();
		},
	};
}
