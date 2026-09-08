// The room's client. It reads what it needs off the mount element, connects,
// reduces, and hands the DOM its three jobs: draw the table, refetch the live
// panels, and draw the debug panel in development.
//
// THERE ARE NO GLOBALS AND NO CONFIGURATION SCRIPT. Everything this module
// needs is an attribute on #tabletop, rendered by the page that knows the
// answers -- which room, which role, which build, and where to connect. A
// closed room renders no socket path, and that is how the client is told not to
// connect.
//
// THE STATE IS CREATED BEFORE THE SOCKET AND OUTLIVES ITS ABSENCE. The renderer
// is a function of the state and a camera, and both of those are worth having
// in a room that is not live: a closed room still has windows to arrange, and a
// room whose socket has not opened yet should show a table rather than a blank
// rectangle that fills in a moment later.

import { announce } from "./panels.ts";
import { empty, reduce } from "./store.ts";
import { Socket, type Status } from "./socket.ts";
import { wireDebug } from "./debug.ts";
import { leaveKicked } from "./exit.ts";
import { mountLayerBar } from "./layer-bar.ts";
import { mountRenderer, type Renderer } from "./render/renderer.ts";
import type { State } from "./protocol.ts";
import { mountWindows } from "./window.ts";

const mount = document.getElementById("tabletop");

if (mount) {
	// The windows do not need a socket. A closed room has no connection and
	// still has a player list worth reading, and the layout somebody arranged
	// should come back whether or not the table is live.
	mountWindows(mount, mount.dataset.room ?? "");

	const state = empty();
	const renderer = mountRenderer(mount, state);

	// The bar's floor control belongs to the renderer's view rather than to the
	// store, because half of what it shows -- which floor this GM is looking at
	// as opposed to which one is active -- exists only in here. It follows the
	// frame that settles it rather than the event, which is one frame earlier
	// and one frame wrong.
	if (renderer) {
		const bar = mountLayerBar(mount, state, renderer);
		if (bar) {
			renderer.onSettled(bar.refresh);
		}
	}

	const path = mount.dataset.socket ?? "";
	if (path !== "") {
		start(path, state, renderer);
	}
}

function start(path: string, state: State, renderer: Renderer | null): void {
	let debug: ReturnType<typeof wireDebug> | null = null;

	const socket = new Socket(path, {
		event(event) {
			// REDUCE FIRST, THEN TELL EVERYBODY. A panel that refetched before
			// the store had applied the event would be reading the server
			// again anyway, but the canvas and the debug panel both read the
			// store -- and an order that put either first would show the room
			// one frame behind for no reason.
			reduce(state, event);
			announce(event);
			debug?.event(event);

			// THE RENDERER IS TOLD RATHER THAN SUBSCRIBED. It holds a reference
			// to the same state object the reducer just mutated, so there is
			// nothing to hand it; all it needs is to know that looking again is
			// worth a frame. Asking on every event is right because the frame
			// loop collapses however many arrive between two frames into one.
			renderer?.invalidate();

			// LAST, AND AFTER THE DEBUG PANEL HAS SEEN IT. This navigates, so
			// nothing below it would run -- and in development the frame that
			// explains why the tab just changed page is worth having in the
			// log first.
			if (event.type === "player.kicked") {
				leaveKicked(event.reason);
			}
		},

		status(status: Status, detail: string) {
			debug?.status(status, detail);
		},
	});

	const panel = document.querySelector("[data-room-debug]");
	if (panel instanceof HTMLElement) {
		debug = wireDebug(panel, socket, state, renderer);
	}

	socket.start();
}
