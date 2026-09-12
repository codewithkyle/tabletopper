import type { Entry } from "./log.ts";
import type { Point } from "../model/types.ts";
import type { RenderStats, Renderer, TextureStats } from "../render/renderer.ts";
import type { Revisions } from "../model/revisions.ts";
import type { SliceDiff } from "./diff.ts";
import type { Socket, SocketStats } from "../socket.ts";
import type { State } from "../protocol.ts";
import type { Tools } from "../tools.ts";
import { cellAt, snapPoint } from "../model/grid.ts";
import { cidOf, describe, incoming, matches, outgoing, pretty } from "./log.ts";
import { diffState } from "./diff.ts";
import { newCounts } from "./counts.ts";
import { normalize } from "../store.ts";
import { reasonNames } from "../render/reasons.ts";
const historyLimit = 200;
const tickMs = 250;
const sizeEveryMs = 1000;
const echoLimit = 20;
const echoStaleMs = 10_000;
const typesShown = 12;
const graphCeiling = 20;
const graphFloor = 4;
const budgetMs = 1000 / 60;
const stressDefault = 500;
export interface DebugDeps {
	socket: Socket;
	state: State;
	revisions: Revisions;
	renderer: Renderer | null;
	tools: Tools | null;
	canvas: HTMLCanvasElement | null;
	user: string;
	viewed(): string;
}
export interface Debug {
	stop(): void;
}
interface Rates {
	at: number;
	drawn: number;
	rebuilds: number;
	spriteEvictions: number;
	tileEvictions: number;
}
export function mountDebug(deps: DebugDeps): Debug {
	const { socket, state, revisions, renderer, tools, canvas, user } = deps;
	const history: Entry[] = [];
	const counts = newCounts();
	const pending = new Map<string, number>();
	const echoes: number[] = [];
	const world: Point = { x: 0, y: 0 };
	let total = 0;
	let logged = 0;
	let logNode: Element | null = null;
	let shownTerm = "";
	let paused = false;
	let connection = "";
	let note = "";
	let stressed = 0;
	let sweeping = false;
	let sized = "";
	let sizedAt = 0;
	let echo = 0;
	let cursor: Point | null = null;
	let checking = false;
	let verdict = "not checked";
	let slices: SliceDiff[] = [];
	let graph: Float64Array = new Float64Array(0);
	let previous: Rates | null = null;
	let fps = 0;
	let rebuildRate = 0;
	let spriteRate = 0;
	let tileRate = 0;
	function add(entry: Entry): void {
		history.push(entry);
		while (history.length > historyLimit) {
			history.shift();
		}
		total++;
		counts.add(`${entry.out ? "> " : "< "}${entry.type}`);
	}
	function settle(cid: string, at: number): void {
		const sent = pending.get(cid);
		if (sent === undefined) {
			return;
		}
		pending.delete(cid);
		echo = at - sent;
		echoes.push(echo);
		while (echoes.length > echoLimit) {
			echoes.shift();
		}
	}
	function settleOldest(at: number): void {
		for (const cid of pending.keys()) {
			settle(cid, at);
			return;
		}
	}
	function prune(at: number): void {
		for (const [cid, sent] of pending) {
			if (at - sent > echoStaleMs) {
				pending.delete(cid);
			}
		}
	}
	socket.watch({
		sent(text) {
			const at = performance.now();
			const cid = cidOf(text);
			if (cid !== "") {
				pending.set(cid, at);
			}
			prune(at);
			add(outgoing(text, at));
		},
		received(text, frame, dropped) {
			const at = performance.now();
			if (frame?.type === "error" && frame.cid !== "") {
				settle(frame.cid, at);
			} else if (frame?.by === user) {
				settleOldest(at);
			}
			if (checking && dropped === null && frame?.type === "snapshot") {
				compare(frame.state, at);
			}
			add(incoming(text, frame, dropped, at));
		},
		status(status, detail) {
			const at = performance.now();
			connection = detail === "" ? status : `${status} - ${detail}`;
			add({
				at,
				out: false,
				type: status,
				detail,
				seq: 0,
				bytes: 0,
				by: "",
				note: "status",
				text: JSON.stringify({ status, detail }),
			});
		},
	});
	function compare(theirs: State, at: number): void {
		checking = false;
		const mine = structuredClone(state);
		const server = structuredClone(theirs);
		normalize(mine);
		normalize(server);
		slices = diffState(mine, server);
		verdict = slices.length === 0
			? `converged at ${Math.round(at / 1000)}s`
			: `${slices.length} slices differ`;
	}
	function pane(name: string): HTMLElement | null {
		const found = document.querySelector(`[data-debug-pane="${name}"]`);
		return found instanceof HTMLElement ? found : null;
	}
	function write(root: HTMLElement, values: Record<string, string>): void {
		for (const node of root.querySelectorAll("[data-debug]")) {
			const value = values[node.getAttribute("data-debug") ?? ""];
			if (value !== undefined) {
				node.textContent = value;
			}
		}
	}
	function paintRenderer(root: HTMLElement): void {
		if (!renderer) {
			write(root, { fps: "no renderer" });
			return;
		}
		const stats = renderer.stats();
		rate(stats);
		write(root, {
			fps: `${fps.toFixed(0)} (${stats.drawn} drawn)`,
			frame: `${stats.last.toFixed(2)}ms`,
			average: `${stats.average.toFixed(2)}ms over ${stats.samples}`,
			p95: `${stats.p95.toFixed(2)}ms`,
			again: stats.again === 0 ? "idle" : reasonNames(stats.again).join(" "),
			calls: String(stats.calls),
			instances: String(stats.instances),
			rebuilds: `${rebuildRate.toFixed(1)}/s (${stats.rebuilds})`,
			painted: String(stats.painted),
			sprites: `${stats.sprites.resident} / ${stats.sprites.capacity}`,
			"sprite-evictions": `${spriteRate.toFixed(1)}/s (${stats.sprites.evictions})`,
			tiles: `${stats.tiles.resident} / ${stats.tiles.capacity}`,
			"tile-evictions": `${tileRate.toFixed(1)}/s (${stats.tiles.evictions})`,
			"sprite-loader": loaderLine(stats.sprites),
			"sprite-missing": missingLine(stats.sprites),
			"tile-loader": loaderLine(stats.tiles),
			"tile-missing": missingLine(stats.tiles),
			zoom: `${stats.zoom.toFixed(3)}x  ${stats.worldPerCssPixel.toFixed(2)} world/px`,
			centre: `${Math.round(stats.x)}, ${Math.round(stats.y)}`,
			level: `z${stats.level}  ${stats.visible} tiles`,
			viewport: `${Math.round(stats.width)} x ${Math.round(stats.height)} css`,
			canvas: `${stats.deviceWidth} x ${stats.deviceHeight} @ ${stats.dpr.toFixed(2)}`,
			losses: String(stats.losses),
			gpu: gpuLine(stats),
			timing: note,
		});
		paintStages(root, stats);
		const surface = root.querySelector("[data-debug-graph]");
		if (surface instanceof HTMLCanvasElement) {
			paintGraph(surface);
		}
		const stress = root.querySelector('[data-debug-action="stress"]');
		if (stress instanceof HTMLButtonElement) {
			stress.textContent = stressed > 0 ? `Stress (${stressed})` : "Stress";
		}
		const benchmark = root.querySelector('[data-debug-action="benchmark"]');
		if (benchmark instanceof HTMLButtonElement) {
			benchmark.disabled = sweeping;
		}
		const timing = root.querySelector('[data-debug-action="timing"]');
		if (timing instanceof HTMLButtonElement) {
			timing.textContent = stats.timing ? "Stop timing" : "Time stages";
		}
	}
	function paintStages(root: HTMLElement, stats: RenderStats): void {
		const list = root.querySelector("[data-debug-stages]");
		const template = root.querySelector("[data-debug-stage]");
		if (!list || !(template instanceof HTMLTemplateElement)) {
			return;
		}
		if (!stats.timing) {
			list.replaceChildren(hint(template, "Timing is off; it costs two clock reads per stage."));
			return;
		}
		const ranked = [...stats.stages].sort((a, b) => (b.build + b.draw) - (a.build + a.draw));
		list.replaceChildren(...ranked.map((stage) => {
			const fragment = template.content.cloneNode(true) as DocumentFragment;
			const name = fragment.querySelector("[data-debug-stage-name]");
			if (name) {
				name.textContent = stage.name;
			}
			const cost = fragment.querySelector("[data-debug-stage-cost]");
			if (cost) {
				cost.textContent = `${stage.build.toFixed(3)} + ${stage.draw.toFixed(3)}ms`;
			}
			return fragment;
		}));
	}
	function hint(template: HTMLTemplateElement, text: string): DocumentFragment {
		const fragment = template.content.cloneNode(true) as DocumentFragment;
		const name = fragment.querySelector("[data-debug-stage-name]");
		if (name) {
			name.textContent = text;
		}
		return fragment;
	}
	function rate(stats: RenderStats): void {
		const at = performance.now();
		if (previous) {
			const seconds = (at - previous.at) / 1000;
			if (seconds > 0) {
				fps = (stats.drawn - previous.drawn) / seconds;
				rebuildRate = (stats.rebuilds - previous.rebuilds) / seconds;
				spriteRate = (stats.sprites.evictions - previous.spriteEvictions) / seconds;
				tileRate = (stats.tiles.evictions - previous.tileEvictions) / seconds;
			}
		}
		previous = {
			at,
			drawn: stats.drawn,
			rebuilds: stats.rebuilds,
			spriteEvictions: stats.sprites.evictions,
			tileEvictions: stats.tiles.evictions,
		};
	}
	function paintGraph(surface: HTMLCanvasElement): void {
		if (!renderer) {
			return;
		}
		const dpr = window.devicePixelRatio || 1;
		const width = Math.max(1, Math.round(surface.clientWidth * dpr));
		const height = Math.max(1, Math.round(surface.clientHeight * dpr));
		if (surface.width !== width || surface.height !== height) {
			surface.width = width;
			surface.height = height;
		}
		const ctx = surface.getContext("2d");
		if (!ctx) {
			return;
		}
		if (graph.length !== width) {
			graph = new Float64Array(width);
		}
		const count = renderer.samples(graph);
		ctx.clearRect(0, 0, width, height);
		if (count === 0) {
			return;
		}
		let peak = graphFloor;
		for (let i = 0; i < count; i++) {
			peak = Math.max(peak, graph[i]);
		}
		const ceiling = Math.max(graphCeiling, peak);
		const step = width / count;
		ctx.strokeStyle = window.getComputedStyle(surface).color;
		ctx.globalAlpha = 0.35;
		ctx.lineWidth = dpr;
		ctx.beginPath();
		const budget = height - (budgetMs / ceiling) * height;
		ctx.moveTo(0, budget);
		ctx.lineTo(width, budget);
		ctx.stroke();
		ctx.globalAlpha = 1;
		ctx.beginPath();
		for (let i = 0; i < count; i++) {
			const y = height - (graph[i] / ceiling) * height;
			const x = i * step;
			if (i === 0) {
				ctx.moveTo(x, y);
			} else {
				ctx.lineTo(x, y);
			}
		}
		ctx.stroke();
	}
	function paintEvents(root: HTMLElement): void {
		const stats = socket.stats();
		write(root, {
			connection: connection,
			sequence: String(socket.sequence()),
			build: socket.build(),
			uptime: stats.since === 0 ? "down" : seconds(performance.now() - stats.since),
			incoming: `${stats.received} frames  ${kilobytes(stats.receivedBytes)}`,
			outgoing: `${stats.sent} frames  ${kilobytes(stats.sentBytes)}`,
			dropped: `${stats.duplicates} duplicate  ${stats.gaps} gaps`,
			resyncs: `${stats.resyncs} after ${stats.connects} connects`,
			echo: echoLine(),
			backoff: stats.attempt === 0 ? "steady" : `attempt ${stats.attempt}, ${Math.round(stats.backoff)}ms`,
			closed: stats.reason === "" ? "never" : stats.reason,
			faults: faultLine(stats),
		});
		const impair = root.querySelector('[data-debug-action="impair"]');
		if (impair instanceof HTMLButtonElement) {
			impair.textContent = socket.impaired() ? "Send cleanly" : "Impair sending";
		}
		paintTypes(root);
		const log = root.querySelector("[data-debug-events]");
		const template = root.querySelector("[data-debug-row]");
		if (log && template instanceof HTMLTemplateElement) {
			paintLog(log, template, root);
		}
		const pause = root.querySelector('[data-debug-action="pause"]');
		if (pause instanceof HTMLButtonElement) {
			pause.textContent = paused ? "Resume" : "Pause";
		}
	}
	function paintTypes(root: HTMLElement): void {
		const list = root.querySelector("[data-debug-types]");
		const template = root.querySelector("[data-debug-type]");
		if (!list || !(template instanceof HTMLTemplateElement)) {
			return;
		}
		const rows = counts.sample(performance.now()).slice(0, typesShown);
		list.replaceChildren(...rows.map((row) => {
			const fragment = template.content.cloneNode(true) as DocumentFragment;
			const name = fragment.querySelector("[data-debug-type-name]");
			if (name) {
				name.textContent = row.type;
			}
			const value = fragment.querySelector("[data-debug-type-count]");
			if (value) {
				value.textContent = `${row.total}  ${row.rate.toFixed(1)}/s`;
			}
			return fragment;
		}));
	}
	function paintLog(node: Element, template: HTMLTemplateElement, root: HTMLElement): void {
		const term = filterTerm(root);
		if (node !== logNode || term !== shownTerm) {
			logNode = node;
			shownTerm = term;
			node.replaceChildren();
			logged = total - history.length;
		}
		if (paused) {
			return;
		}
		const fresh = Math.min(total - logged, history.length);
		for (let i = history.length - fresh; i < history.length; i++) {
			if (matches(history[i], term)) {
				node.prepend(row(template, history[i]));
			}
		}
		while (node.childElementCount > historyLimit) {
			node.lastElementChild?.remove();
		}
		logged = total;
	}
	function row(template: HTMLTemplateElement, entry: Entry): DocumentFragment {
		const fragment = template.content.cloneNode(true) as DocumentFragment;
		const line = fragment.querySelector("[data-debug-row-line]");
		if (line) {
			line.textContent = describe(entry);
		}
		const json = fragment.querySelector("[data-debug-row-json]");
		if (json) {
			json.textContent = entry.text;
		}
		return fragment;
	}
	function filterTerm(root: HTMLElement): string {
		const input = root.querySelector("[data-debug-filter]");
		return input instanceof HTMLInputElement ? input.value.trim() : "";
	}
	function echoLine(): string {
		if (echoes.length === 0) {
			return "nothing sent yet";
		}
		let sum = 0;
		for (const one of echoes) {
			sum += one;
		}
		return `${echo.toFixed(0)}ms last, ${(sum / echoes.length).toFixed(0)}ms over ${echoes.length}`;
	}
	function paintState(root: HTMLElement): void {
		let points = 0;
		for (const stroke of state.strokes) {
			points += stroke.points.length / 2;
		}
		const now = performance.now();
		if (sized === "" || now - sizedAt > sizeEveryMs) {
			sized = kilobytes(JSON.stringify(state).length);
			sizedAt = now;
		}
		write(root, {
			"active-layer": layerLabel(state.table.activeLayer),
			"viewed-layer": layerLabel(deps.viewed()),
			tool: tools ? `${tools.chosen()} / ${tools.mode()}` : "none",
			cursor: cursorLine(),
			"count-players": String(state.players.length),
			"count-pawns": String(state.pawns.length),
			"count-layers": String(state.table.layers.length),
			"count-fog": String(state.fog.length),
			"count-strokes": String(state.strokes.length),
			"count-points": String(points),
			"count-initiative": `${state.initiative.entries.length} round ${state.initiative.round}`,
			"state-bytes": sized,
			"rev-pawns": String(revisions.pawns),
			"rev-table": String(revisions.table),
			"rev-fog": String(revisions.fog),
			"rev-strokes": String(revisions.strokes),
			"rev-initiative": String(revisions.initiative),
			desync: checking ? "waiting for a snapshot" : verdict,
		});
		const players = root.querySelector("[data-debug-players]");
		if (players) {
			players.replaceChildren(...state.players.map((player) => {
				const line = document.createElement("li");
				line.textContent = `${player.name} (${player.role})${player.connected ? "" : " - away"}`;
				return line;
			}));
		}
		paintSlices(root);
	}
	function paintSlices(root: HTMLElement): void {
		const list = root.querySelector("[data-debug-slices]");
		const template = root.querySelector("[data-debug-slice]");
		if (!list || !(template instanceof HTMLTemplateElement)) {
			return;
		}
		list.replaceChildren(...slices.map((slice) => {
			const fragment = template.content.cloneNode(true) as DocumentFragment;
			const name = fragment.querySelector("[data-debug-slice-name]");
			if (name) {
				name.textContent = slice.name;
			}
			const detail = fragment.querySelector("[data-debug-slice-detail]");
			if (detail) {
				detail.textContent = slice.detail;
			}
			return fragment;
		}));
	}
	function cursorLine(): string {
		if (!cursor || !renderer || !canvas) {
			return "off the table";
		}
		const rect = canvas.getBoundingClientRect();
		renderer.toWorld(cursor.x - rect.left, cursor.y - rect.top, world);
		const grid = state.table.grid;
		const [cx, cy] = cellAt(grid, world.x, world.y);
		const [sx, sy] = snapPoint(grid, 1, 1, world.x, world.y);
		return `${Math.round(world.x)}, ${Math.round(world.y)}  cell ${cx}, ${cy}  snap ${Math.round(sx)}, ${Math.round(sy)}`;
	}
	function layerLabel(id: string): string {
		if (id === "") {
			return "none";
		}
		const layer = state.table.layers.find((one) => one.id === id);
		return layer ? `${layer.name} ${id.slice(-6)}` : id.slice(-6);
	}
	function paint(): void {
		const rendering = pane("renderer");
		if (rendering) {
			paintRenderer(rendering);
		}
		const events = pane("events");
		if (events) {
			paintEvents(events);
		}
		const counted = pane("state");
		if (counted) {
			paintState(counted);
		}
	}
	function stressCount(): number {
		const parsed = Math.round(number("[data-debug-stress-count]", stressDefault));
		return parsed > 0 ? parsed : stressDefault;
	}
	function number(selector: string, fallback: number): number {
		const input = document.querySelector(selector);
		if (!(input instanceof HTMLInputElement)) {
			return fallback;
		}
		const parsed = Number.parseFloat(input.value);
		return Number.isFinite(parsed) ? parsed : fallback;
	}
	function act(action: string): void {
		switch (action) {
			case "skip":
				socket.skip(Math.round(number("[data-debug-skip-count]", 1)));
				return;
			case "reconnect":
				note = socket.disconnect() ? "connection dropped" : "already down";
				return;
			case "impair":
				socket.impair(socket.impaired() ? null : {
					latency: Math.max(0, number("[data-debug-latency]", 0)),
					jitter: Math.max(0, number("[data-debug-jitter]", 0)),
					loss: Math.min(1, Math.max(0, number("[data-debug-loss]", 0) / 100)),
				});
				return;
			case "timing":
				renderer?.timing(!(renderer.stats().timing));
				return;
			case "pause":
				paused = !paused;
				return;
			case "clear":
				history.length = 0;
				total = 0;
				logged = 0;
				logNode = null;
				counts.reset();
				return;
			case "copy":
				void navigator.clipboard?.writeText(history.map((entry) => entry.text).join("\n"));
				return;
			case "check":
				slices = [];
				if (!socket.stats().open) {
					verdict = "not connected";
					return;
				}
				checking = true;
				socket.resync();
				return;
			case "benchmark":
				if (!renderer) {
					note = "no renderer";
					return;
				}
				sweeping = true;
				note = "sweeping...";
				renderer.benchmark((result) => {
					sweeping = false;
					note = `avg ${result.average.toFixed(2)}ms  p95 ${result.p95.toFixed(2)}ms  ` +
						`${result.frames} frames  ${result.tiles} tiles`;
				});
				return;
			case "stress":
				stressed = renderer ? renderer.stress(stressed > 0 ? 0 : stressCount()) : 0;
				return;
			case "lose-context":
				note = renderer?.loseContext() ? "context dropped" : "WEBGL_lose_context unavailable";
				return;
		}
	}
	function onClick(e: Event): void {
		if (!(e.target instanceof Element)) {
			return;
		}
		const summary = e.target.closest("summary");
		const json = summary?.parentElement?.querySelector("[data-debug-row-json]");
		if (json instanceof HTMLElement) {
			json.textContent = pretty(json.textContent ?? "");
		}
		const button = e.target.closest("[data-debug-action]");
		if (!(button instanceof HTMLButtonElement)) {
			return;
		}
		act(button.dataset.debugAction ?? "");
		paint();
	}
	function onSubmit(e: Event): void {
		if (!(e.target instanceof HTMLFormElement) || !e.target.matches("[data-debug-form]")) {
			return;
		}
		e.preventDefault();
		const input = e.target.querySelector("[data-debug-input]");
		if (!(input instanceof HTMLTextAreaElement)) {
			return;
		}
		const text = input.value.trim();
		if (text !== "" && socket.raw(text)) {
			input.value = "";
		}
	}
	function onPointerMove(e: PointerEvent): void {
		cursor = { x: e.clientX, y: e.clientY };
	}
	function onPointerLeave(): void {
		cursor = null;
	}
	document.addEventListener("click", onClick);
	document.addEventListener("submit", onSubmit);
	canvas?.addEventListener("pointermove", onPointerMove);
	canvas?.addEventListener("pointerleave", onPointerLeave);
	const ticker = setInterval(paint, tickMs);
	return {
		stop() {
			clearInterval(ticker);
			socket.watch(null);
			document.removeEventListener("click", onClick);
			document.removeEventListener("submit", onSubmit);
			canvas?.removeEventListener("pointermove", onPointerMove);
			canvas?.removeEventListener("pointerleave", onPointerLeave);
		},
	};
}
function loaderLine(stats: TextureStats): string {
	const loader = stats.loader;
	return `${loader.inFlight} live  ${loader.queued} queued  ${loader.ready} ready`;
}
function missingLine(stats: TextureStats): string {
	const loader = stats.loader;
	return `${loader.missing} gone  ${loader.failed} retrying  ${loader.fetched} fetched`;
}
function kilobytes(bytes: number): string {
	return `${(bytes / 1024).toFixed(1)} KB`;
}
function seconds(ms: number): string {
	const whole = Math.floor(ms / 1000);
	return whole < 60 ? `${whole}s` : `${Math.floor(whole / 60)}m ${whole % 60}s`;
}
function gpuLine(stats: RenderStats): string {
	if (!stats.gpuAvailable) {
		return "EXT_disjoint_timer_query_webgl2 unavailable";
	}
	if (!stats.timing) {
		return "timing off";
	}
	return stats.gpu < 0 ? "waiting for a query" : `${stats.gpu.toFixed(3)}ms`;
}
function faultLine(stats: SocketStats): string {
	if (stats.skipping === 0 && stats.lost === 0 && stats.delayed === 0) {
		return "none injected";
	}
	return `${stats.skipping} to skip  ${stats.lost} dropped  ${stats.delayed} delayed`;
}
