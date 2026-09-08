// The room's client. It reads what it needs off the mount element, connects,
// reduces, and hands the DOM its two jobs: refetch the live panels, and draw
// the debug panel in development.
//
// THERE ARE NO GLOBALS AND NO CONFIGURATION SCRIPT. Everything this module
// needs is an attribute on #tabletop, rendered by the page that knows the
// answers -- which room, which role, which build, and where to connect. A
// closed room renders no socket path, and that is how the client is told not to
// connect.
//
// THE CANVAS IS NOT HERE YET. What is here is the protocol on a socket, and a
// debug panel that proves it; the renderer imports the same store when it
// arrives and reads the state this keeps.

import { announce } from "./panels.ts";
import { empty, reduce } from "./store.ts";
import { Socket, type Status } from "./socket.ts";
import { wireDebug } from "./debug.ts";

const mount = document.getElementById("tabletop");
const path = mount?.dataset.socket ?? "";

if (mount && path !== "") {
	start(mount, path);
}

function start(mount: HTMLElement, path: string): void {
	const state = empty();

	let debug: ReturnType<typeof wireDebug> | null = null;

	const socket = new Socket(path, {
		event(event) {
			// REDUCE FIRST, THEN TELL EVERYBODY. A panel that refetched before
			// the store had applied the event would be reading the server
			// again anyway, but the debug panel reads the store -- and an
			// order that put it first would show the room one frame behind for
			// no reason.
			reduce(state, event);
			announce(event);
			debug?.event(event);
		},

		status(status: Status, detail: string) {
			debug?.status(status, detail);
		},
	});

	const panel = document.querySelector("[data-room-debug]");
	if (panel instanceof HTMLElement) {
		debug = wireDebug(panel, socket, state);
	}

	socket.start();
}
