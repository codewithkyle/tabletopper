// The development panel: what the connection is doing, where the sequence is,
// who the client thinks is in the room, the last twenty frames, and a box to
// send a command by hand.
//
// IT IS ONLY RENDERED IN DEVELOPMENT. RoomPageData.Debug is the config's
// Development flag and nothing else, so the markup this fills is simply absent
// in production and every query below finds nothing.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE, and none may be. server/js is bundled
// by esbuild and is not a Tailwind source, so a class named here would never be
// emitted -- the panel's whole appearance is rendered in templ and this sets
// text and toggles [hidden].

import type { Event, State } from "./protocol.ts";
import type { Renderer } from "./render/renderer.ts";
import type { Socket, Status } from "./socket.ts";

const historyLimit = 20;

export function wireDebug(root: HTMLElement, socket: Socket, state: State, renderer: Renderer | null): {
	status(status: Status, detail: string): void;
	event(event: Event): void;
} {
	const connection = root.querySelector("[data-debug-connection]");
	const sequence = root.querySelector("[data-debug-seq]");
	const build = root.querySelector("[data-debug-version]");
	// The active layer, because half the commands worth typing into the box
	// below name one and there is nowhere else on the page to read it from.
	const layer = root.querySelector("[data-debug-layer]");
	const players = root.querySelector("[data-debug-players]");
	const events = root.querySelector("[data-debug-events]");
	const form = root.querySelector("[data-debug-form]");
	const input = root.querySelector("[data-debug-input]");

	// THE BENCHMARK IS THE ONE MEASUREMENT THE ARCHITECTURE ASKED FOR. It
	// sweeps the camera on a script -- three passes across the map at three
	// zooms, then a zoom in and out at the centre -- so that two runs on two
	// machines are comparable and a change that made the renderer slower shows
	// up as a number rather than as a feeling.
	//
	// WHAT IT REPORTS IS CPU TIME INSIDE THE RENDER, not the frame interval.
	// The interval belongs to the display and is 16.7ms on a machine doing
	// nothing at all; what this renderer controls is how much of it is spent
	// here, and the budget is a fraction of a millisecond.
	const benchmark = root.querySelector("[data-debug-benchmark]");
	const timing = root.querySelector("[data-debug-timing]");

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

		// The frame is written out as it arrived, because the whole point of a
		// text protocol is that it can be read.
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
