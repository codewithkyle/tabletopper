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

// The colour picker registers <hex-alpha-color-picker> at module scope, so
// the import is here rather than in color.ts -- node runs the tests beside
// that file and has no custom element registry to define into.
import "vanilla-colorful/hex-alpha-color-picker.js";

import { announce } from "./panels.ts";
import { createTable } from "./pawns.ts";
import { empty, reduce } from "./store.ts";
import { mountDialogs } from "./dialogs.ts";
import { mountOverlay } from "./overlay.ts";
import { Socket, type Status } from "./socket.ts";
import { wireDebug } from "./debug.ts";
import { leaveKicked } from "./exit.ts";
import { mountColorFields } from "./color.ts";
import { mountHitPoints } from "./hp.ts";
import { mountLayerBar } from "./layer-bar.ts";
import { mountPawnMenu } from "./pawn-menu.ts";
import { mountLayerTool } from "./layer-tool.ts";
import { mountRenderer, type Renderer } from "./render/renderer.ts";
import { mountTools } from "./tools.ts";
import type { Event, Role, State } from "./protocol.ts";
import { mountWindows, openWindow } from "./window.ts";
import { pawnWindow } from "./pawn-window.ts";
import type { Named } from "./pawn-window.ts";
import type { Overlay } from "./overlay.ts";
import type { Table } from "./pawns.ts";

const mount = document.getElementById("tabletop");

if (mount) {
	// The windows do not need a socket. A closed room has no connection and
	// still has a player list worth reading, and the layout somebody arranged
	// should come back whether or not the table is live.
	mountWindows(mount, mount.dataset.room ?? "");

	// The hit-point boxes inside those windows. It is one listener on the
	// document rather than anything a panel owns, because a panel is markup
	// htmx swapped in and has no mount of its own to run.
	mountHitPoints();

	// And the grid colour, for the same reason and in the same way: the picker
	// arrives inside a window nothing here rendered.
	mountColorFields();

	// The tool pill. It is mounted before anything that reads it, needs neither
	// a socket nor a canvas, and a closed room still switches modes with it --
	// which is the same reason the windows are mounted above.
	const tools = mountTools(mount);

	const state = empty();
	const roomID = mount.dataset.room ?? "";
	const role: Role = mount.dataset.role === "gm" ? "gm" : "player";
	const user = mount.dataset.user ?? "";

	// ONE WAY INTO A PAWN'S WINDOW AND NOT TWO. A double click opens it and so
	// does the first item on the right-click menu; pawn-window.ts is the file
	// that says what one of those windows IS, and an id spelled differently in
	// either place is a window that opens twice and never closes.
	const openDetails = (pawn: Named): void => {
		openWindow(pawnWindow(roomID, pawn));
	};

	// The floors menu in that pill, which is the GM's and renders nothing at all
	// for anybody else. It reads the store rather than the renderer: which floor
	// is ACTIVE is the room's, and the local choice of which one to look at
	// belongs to the control in the bar.
	mountLayerTool(mount, state);

	// The menu the right button puts up. It is mounted before the table because
	// the table is what asks for it, and it needs nothing the renderer holds:
	// the point it opens at arrives with the question.
	const menu = mountPawnMenu(mount, {
		layers: () => state.table.layers.map((layer) => ({ id: layer.id, name: layer.name })),
		details: openDetails,
	});

	// THE THREE OF THESE REFER TO EACH OTHER AND THE CYCLE IS BROKEN WITH
	// CALLBACKS RATHER THAN WITH ORDER. The table needs a socket to send a move
	// and a renderer to know which floor is being looked at; the renderer needs
	// the table to give it a tool; the socket needs the table to hand it a
	// drag. Every one of those is a function call at the time it happens, so
	// each is a closure over a `let` rather than a constructor argument.
	let socket: Socket | null = null;
	let renderer: Renderer | null = null;
	let overlay: Overlay | null = null;

	const table = createTable({
		state,
		role,
		user,

		// THE VIEWED FLOOR IS THE RENDERER'S AND NOT THE STORE'S, because the
		// GM's local choice to look at another floor exists only in there. A
		// player has no such choice and falls back to the active layer, which
		// is the only one they hold pawns for anyway.
		viewed: () => renderer?.view.viewed()?.id ?? state.table.activeLayer,
		send: (command) => {
			socket?.send(command);
		},
		invalidate: () => renderer?.invalidate(),

		// WHICH MODE THE PILL IS IN, ASKED AT EVERY PRESS. A page that rendered
		// no pill answers no, which is the table behaving as it always has
		// rather than a table nothing can be done to.
		panning: () => tools?.panning() ?? false,

		// AND WHETHER THE RULER IS THE MODE, which is the same question asked of
		// the same pill and is deliberately not the same answer: the space bar
		// borrows the pointer without changing which tool is chosen, so a
		// measurement survives being panned across. See tools.ts.
		measuring: () => tools?.measuring() ?? false,

		// A CAMERA THAT HAS NOT STARTED IS ONE MAP PIXEL PER SCREEN PIXEL,
		// which is the identity rather than a guess: with no renderer there is
		// no canvas, so nothing asks for a handle and the number is never used.
		scale: () => renderer?.mapPerPixel() ?? 1,

		// A DOUBLE CLICK ON A PAWN OPENS ITS WINDOW, and this is where the room
		// id lives. The canvas knows which pawn; pawn-window.ts knows what one
		// of these windows is; neither of them knows which table is being
		// looked at.
		details: openDetails,

		// AND THE RIGHT BUTTON ASKS THE MENU, which is the other half of the
		// same split: the canvas found the pawn and worked out where on screen
		// the question was asked, and everything about what a menu offers --
		// the floors, the removal, who may see which -- belongs to markup that
		// has the room in it.
		menu: (pawn, screen) => {
			menu?.open(pawn, screen);
		},

		// AND DELETE PRESSES THE OVERLAY'S OWN BUTTON, which is what carries the
		// hx-confirm. See overlay.ts: the confirmation belongs to the element
		// making the request, so the way to get it is to press that element
		// rather than to build the DELETE here.
		remove: () => overlay?.remove(),
	});

	renderer = mountRenderer(mount, state, table);

	// A MODE CHANGED WITH THE POINTER SITTING STILL IS STILL A FRAME. The measure
	// tool leaves a ruler on the table and choosing another tool is what puts it
	// away -- and nothing else would ask for the frame that stops drawing it,
	// because the pill is a button in the corner and the canvas hears nothing
	// about it.
	tools?.onChange(() => renderer?.invalidate());

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

		const view = renderer;
		overlay = mountOverlay(mount, {
			focus: () => table.focus(),
			selected: () => table.selection.ids(),
			bounds: () => table.bounds(),
			project: (x, y, out) => view.toScreen(x, y, out),
			layers: () => state.table.layers.map((layer) => ({ id: layer.id, name: layer.name })),

			// A PLAYER'S STORE NEVER HOLDS A HIDDEN PAWN, so this reads true
			// for anything they could have selected. It costs nothing to be
			// right there anyway: the button it answers for is the GM's, and
			// a player's page renders none.
			anyShown: (ids) => state.pawns.some((pawn) => pawn.visible && ids.includes(pawn.id)),
			labels: () => state.table.pawnLabels,
		});

		if (overlay) {
			// WHAT IT SAYS ON A CHANGE AND WHERE IT IS ON EVERY FRAME. Those
			// are different rates and writing either at the other's would be
			// wrong in a different way; see overlay.ts.
			table.onChange(overlay.refresh);
			view.onFrame(overlay.place);
		}
	}

	// The spawn dialog's own behaviour, and the bridge that turns a picked card
	// into a canvas that is armed. It needs no socket and no canvas: a room
	// whose renderer would not start still opens the dialog, and arming lands
	// nowhere, which is the right amount of nothing to happen.
	mountDialogs(table.arm);

	const path = mount.dataset.socket ?? "";
	if (path !== "") {
		socket = start(path, state, renderer, table, overlay);
	}
}

// touchesPawns is which events move something the pawn pass has already put in
// its buffer. A snapshot replaces the whole table; the pawn family is itself;
// table.updated carries the grid, whose cell size is every pawn's radius.
//
// pawn.dragging is deliberately absent. It is a preview nobody has committed
// to, it is drawn from a buffer of its own, and it is one of the three hot paths
// in the protocol -- rebuilding the whole table's instances twenty times a
// second for a ghost is exactly what the split between the two buffers exists to
// avoid.
function touchesPawns(type: Event["type"]): boolean {
	return type === "snapshot" || type === "table.updated" || (type.startsWith("pawn.") && type !== "pawn.dragging");
}

function start(
	path: string,
	state: State,
	renderer: Renderer | null,
	table: Table,
	overlay: Overlay | null,
): Socket {
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

			// AND THE PAWNS ARE TOLD SEPARATELY, because their instance buffer
			// is rebuilt on a change rather than per frame -- a pan is the
			// common case by a wide margin and rebuilds nothing. The reducer
			// mutates the store in place, which is what keeps a pawn moving
			// from allocating and is also why there is nothing to subscribe to;
			// this is the subscription, in one line, at the one place that
			// knows an event happened.
			if (touchesPawns(event.type)) {
				renderer?.pawnsChanged();

				// AND THE PANEL OVER THE TABLE, which is otherwise rewritten
				// only when the pointer does something. What it says is read
				// off the pawns -- the hovered one's hit points, and whether
				// the selection is hidden or shown -- so a change that arrived
				// over the wire leaves it describing a pawn as it was: a
				// goblin still at full health after somebody else hit it, and
				// a Hide button that has already been pressed.
				overlay?.refresh();
			}

			// WIPING A FLOOR WIPES ITS BLOOD. The marks the canvas puts down
			// when somebody is hurt are the viewer's own -- they are never sent
			// anywhere and no event carries them -- so this is the only way they
			// are ever cleared other than a reload. stroke.cleared is the right
			// event for it twice over: it is what the GM's "wipe the drawing"
			// sends, and TableClear sends one per floor, so packing the table
			// away at the end of an evening cleans it.
			if (event.type === "stroke.cleared") {
				renderer?.bloodCleared(event.layer);
			}

			// AND A SNAPSHOT IS NOT A ROUND OF COMBAT. It arrives on the first
			// join and on every reconnect, and what it carries is the table as
			// it is NOW -- which, after a laptop lid has been shut for three
			// rounds, differs from what the canvas remembers by everything that
			// happened in them. Bleeding for that difference would put one
			// enormous mark on the floor for a fight that took place while
			// nobody was looking, so the marks already down stay and the
			// comparison starts again from here.
			if (event.type === "snapshot") {
				renderer?.bloodResync();
			}

			// SOMEBODY ELSE'S DRAG, AND WHAT ENDS ONE. The ghosts other people
			// are dragging live outside the store on purpose -- pawn.dragging
			// is transient and the reducer never sees it -- so this is the one
			// consumer, and it also hears the committed moves that tell it a
			// preview is over.
			table.preview(event);

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

	return socket;
}
