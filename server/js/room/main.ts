import "vanilla-colorful/hex-alpha-color-picker.js";
import "vanilla-colorful/hex-color-picker.js";
import { ALERT, SETTINGS_CHANGE } from "../../public/js/events.js";
import { announce } from "./panels.ts";
import { fanOut, refusals, touchesPawns } from "./effects.ts";
import { createDraw } from "./draw.ts";
import { mountDrawTool } from "./draw-tool.ts";
import { FULL, newPingSound } from "./ping-sound.ts";
import { createFog } from "./fog.ts";
import { actorColor, createTable, hexColor } from "./pawns.ts";
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
import { mountEntryMenu } from "./initiative-menu.ts";
import { mountLayeredMenu } from "./layered-menu.ts";
import { mountFogTool } from "./fog-tool.ts";
import { mountFollow, type Follow } from "./follow.ts";
import { mountTurns, type Turns } from "./initiative.ts";
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
	mountWindows(mount, mount.dataset.room ?? "");
	mountHitPoints();
	mountColorFields();
	const tools = mountTools(mount);
	const state = empty();
	const roomID = mount.dataset.room ?? "";
	const role: Role = mount.dataset.role === "gm" ? "gm" : "player";
	const user = mount.dataset.user ?? "";
	const openDetails = (pawn: Named): void => {
		openWindow(pawnWindow(roomID, pawn));
	};
	mountLayerTool(mount, state);
	const turns: Turns | null = mountTurns(mount, state);
	mountEntryMenu(mount, { details: openDetails });
	const menu = mountPawnMenu(mount, {
		layers: () => state.table.layers.map((layer) => ({ id: layer.id, name: layer.name })),
		details: openDetails,
	});
	let socket: Socket | null = null;
	let renderer: Renderer | null = null;
	const fogTool = mountFogTool(mount, tools);
	const drawTool = mountDrawTool(mount, tools, hexColor(actorColor(user)));
	const sound = newPingSound();
	const rendered = Number.parseInt(mount.dataset.pingVolume ?? "", 10);
	sound.volume(Number.isFinite(rendered) ? rendered : FULL);
	let overlay: Overlay | null = null;
	let follow: Follow | null = null;
	const viewed = () => renderer?.view.viewed()?.id ?? state.table.activeLayer;
	const fog = createFog({
		state,
		role,
		user,
		viewed,
		grid: () => state.table.grid,
		send: (command) => {
			socket?.send(command);
		},
		invalidate: () => renderer?.invalidate(),
		fogging: () => tools?.fogging() ?? false,
		options: fogTool.options,
	});
	const draw = createDraw({
		state,
		role,
		user,
		viewed,
		grid: () => state.table.grid,
		send: (command) => {
			socket?.send(command);
		},
		invalidate: () => renderer?.invalidate(),
		scale: () => renderer?.mapPerPixel() ?? 1,
		drawing: () => tools?.drawing() ?? false,
		options: drawTool.options,
	});
	const table = createTable({
		state,
		role,
		user,
		fog,
		draw,
		viewed,
		send: (command) => {
			socket?.send(command);
		},
		invalidate: () => renderer?.invalidate(),
		panning: () => tools?.panning() ?? false,
		measuring: () => tools?.measuring() ?? false,
		pinging: () => tools?.pinging() ?? false,
		scale: () => renderer?.mapPerPixel() ?? 1,
		details: openDetails,
		menu: (pawn, screen) => {
			menu?.open(pawn, screen);
		},
		remove: () => overlay?.remove(),
	});
	renderer = mountRenderer(mount, state, role, table);
	tools?.onChange(() => renderer?.invalidate());
	if (renderer) {
		const bar = mountLayerBar(mount, state, renderer);
		if (bar) {
			renderer.onSettled(bar.refresh);
		}
		const layered = mountLayeredMenu(viewed);
		if (layered) {
			renderer.onSettled(layered.refresh);
		}
		renderer.onSettled(() => table.floorChanged());
		const view = renderer;
		overlay = mountOverlay(mount, {
			focus: () => table.focus(),
			selected: () => table.selection.ids(),
			bounds: () => table.bounds(),
			project: (x, y, out) => view.toScreen(x, y, out),
			layers: () => state.table.layers.map((layer) => ({ id: layer.id, name: layer.name })),
			anyShown: (ids) => state.pawns.some((pawn) => pawn.visible && ids.includes(pawn.id)),
			labels: () => state.table.pawnLabels,
		});
		if (overlay) {
			table.onChange(overlay.refresh);
			view.onFrame(overlay.place);
		}
		follow = mountFollow(state, {
			viewed: () => view.view.viewed()?.id ?? state.table.activeLayer,
			focus: (rect) => view.focus(rect),
		});
		follow.following(mount.dataset.followTurn !== undefined);
		view.showBlood(mount.dataset.showBlood !== undefined);
		window.addEventListener(SETTINGS_CHANGE, (e) => {
			const detail = (e as CustomEvent<{
				followTurn?: boolean;
				showBlood?: boolean;
				pingVolume?: number;
			}>).detail;
			follow?.following(detail?.followTurn !== false);
			view.showBlood(detail?.showBlood !== false);
			if (typeof detail?.pingVolume === "number") {
				sound.volume(detail.pingVolume);
			}
		});
	}
	mountDialogs(table.arm);
	const pinged = (layer: string, x: number, y: number, by: string) => {
		renderer?.pinged(layer, x, y, by);
		if (by !== user && layer === viewed()) {
			sound.play();
		}
	};
	const path = mount.dataset.socket ?? "";
	if (path !== "") {
		socket = start(path, state, renderer, table, overlay, turns, follow, pinged);
	}
}
function start(
	path: string,
	state: State,
	renderer: Renderer | null,
	table: Table,
	overlay: Overlay | null,
	turns: Turns | null,
	follow: Follow | null,
	pinged: (layer: string, x: number, y: number, by: string) => void,
): Socket {
	let debug: ReturnType<typeof wireDebug> | null = null;
	let socket: Socket | null = null;
	const effect = fanOut([
		(event) => reduce(state, event),
		announce,
		(event) => debug?.event(event),
		() => renderer?.invalidate(),
		(event) => {
			if (touchesPawns(event.type)) {
				renderer?.pawnsChanged();
				overlay?.refresh();
			}
		},
		(event) => {
			if (event.type === "stroke.cleared") {
				renderer?.bloodCleared(event.layer);
			}
		},
		(event) => {
			if (event.type === "pinged") {
				pinged(event.layer, event.x, event.y, event.by ?? "");
			}
		},
		(event) => {
			if (event.type === "snapshot") {
				renderer?.bloodResync();
			}
		},
		(event) => {
			if (event.type === "snapshot" || event.type === "initiative.updated") {
				turns?.changed();
			}
		},
		(event) => follow?.event(event),
		(event) => table.preview(event),
		refusals({
			alert: (heading, message) => {
				window.dispatchEvent(new CustomEvent(ALERT, { detail: { heading, message } }));
			},
			resync: () => socket?.resync(),
		}),
		(event) => {
			if (event.type === "player.kicked") {
				leaveKicked(event.reason);
			}
		},
	]);
	socket = new Socket(path, {
		event: effect,
		status(status: Status, detail: string) {
			debug?.status(status, detail);
			if (status !== "ended") {
				return;
			}
			if (detail === "left") {
				location.assign("/");
			}
			if (detail === "limit") {
				window.dispatchEvent(new CustomEvent(ALERT, {
					detail: {
						heading: "Too many tabs",
						message: "You already have as many connections to this room as one person may hold. Close another tab and reload this one.",
					},
				}));
			}
		},
	});
	const panel = document.querySelector("[data-room-debug]");
	if (panel instanceof HTMLElement) {
		debug = wireDebug(panel, socket, state, renderer);
	}
	socket.start();
	return socket;
}
