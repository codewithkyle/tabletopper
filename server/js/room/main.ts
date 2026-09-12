import "vanilla-colorful/hex-alpha-color-picker.js";
import "vanilla-colorful/hex-color-picker.js";
import { ALERT, SETTINGS_CHANGE } from "../../public/js/events.js";
import { announce } from "./panels.ts";
import { fanOut, refusals } from "./effects.ts";
import { mountDrawTool } from "./draw-tool.ts";
import { FULL, newPingSound } from "./ping-sound.ts";
import { createTable } from "./modes/table.ts";
import { actorColor, hexColor } from "./model/color.ts";
import { empty, reduce } from "./store.ts";
import { revise, revisions, watching } from "./model/revisions.ts";
import { mountDialogs } from "./dialogs.ts";
import { mountHud } from "./hud.ts";
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
import type { Role, State } from "./protocol.ts";
import type { Revisions } from "./model/revisions.ts";
import { mountWindows, openWindow } from "./window.ts";
import { pawnWindow } from "./pawn-window.ts";
import type { Named } from "./pawn-window.ts";
import type { Hud } from "./hud.ts";
import type { Table } from "./modes/table.ts";
const mount = document.getElementById("tabletop");
if (mount) {
	mountWindows(mount, mount.dataset.room ?? "");
	mountHitPoints();
	mountColorFields();
	const tools = mountTools(mount);
	const state = empty();
	const rev = revisions();
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
	let hud: Hud | null = null;
	let follow: Follow | null = null;
	const viewed = () => renderer?.view.viewed()?.id ?? state.table.activeLayer;
	const table = createTable({
		state,
		role,
		user,
		viewed,
		send: (command) => {
			socket?.send(command);
		},
		invalidate: () => renderer?.invalidate(),
		scale: () => renderer?.mapPerPixel() ?? 1,
		details: openDetails,
		menu: (pawn, screen) => {
			menu?.open(pawn, screen);
		},
		remove: () => hud?.remove(),
		mode: () => tools?.mode() ?? "select",
		chosen: () => tools?.chosen() ?? "select",
		fogOptions: fogTool.options,
		drawOptions: drawTool.options,
	});
	renderer = mountRenderer(mount, state, rev, role, user, table.tool);
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
		hud = mountHud(mount, {
			focus: () => table.focus(),
			selected: () => table.selection.ids(),
			bounds: () => table.bounds(),
			project: (x, y, out) => view.toScreen(x, y, out),
			layers: () => state.table.layers.map((layer) => ({ id: layer.id, name: layer.name })),
			anyShown: (ids) => state.pawns.some((pawn) => pawn.visible && ids.includes(pawn.id)),
			labels: () => state.table.pawnLabels,
		});
		if (hud) {
			table.onChange(hud.refresh);
			view.onFrame(hud.place);
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
	const pinged = (layer: string, by: string) => {
		if (by !== user && layer === viewed()) {
			sound.play();
		}
	};
	const path = mount.dataset.socket ?? "";
	if (path !== "") {
		socket = start(path, state, rev, renderer, table, hud, turns, follow, pinged);
	}
}
function start(
	path: string,
	state: State,
	rev: Revisions,
	renderer: Renderer | null,
	table: Table,
	hud: Hud | null,
	turns: Turns | null,
	follow: Follow | null,
	pinged: (layer: string, by: string) => void,
): Socket {
	let debug: ReturnType<typeof wireDebug> | null = null;
	let socket: Socket | null = null;
	const shown = watching(["pawns", "table", "fog"]);
	const effect = fanOut([
		(event) => {
			reduce(state, event);
			revise(rev, event);
		},
		(event) => renderer?.event(event),
		() => {
			if (shown.changed(rev)) {
				hud?.refresh();
			}
		},
		(event) => {
			if (event.type === "pinged") {
				pinged(event.layer, event.by ?? "");
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
		frame(frame) {
			if (frame.type === "changes") {
				for (const change of frame.events) {
					effect(change);
				}
			} else {
				effect(frame);
			}
			announce(frame);
			debug?.frame(frame);
		},
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
