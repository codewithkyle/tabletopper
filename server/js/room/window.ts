import { WINDOW_CLOSE, WINDOW_RETITLE } from "../../public/js/events.js";
import { read, store } from "./model/storage.ts";
const SNAP = 10;
const MIN_WIDTH = 200;
const MIN_HEIGHT = 120;
const DEFAULT_WIDTH = 280;
const DEFAULT_HEIGHT = 320;
const RECLAMP_DELAY = 150;
const FRAGMENT_PREFIX = "/fragment/";
declare const htmx: {
	ajax(verb: string, path: string, context: { target: Element; source: Element }): void;
};
export interface WindowSpec {
	id: string;
	title: string;
	url: string;
	width?: number;
	height?: number;
}
interface Geometry {
	x: number;
	y: number;
	w: number;
	h: number;
}
interface RequestDetail {
	ctx: { response?: { status?: number } };
}
let table: HTMLElement | null = null;
let shell: HTMLTemplateElement | null = null;
let room = "";
const windows = new Map<string, RoomWindow>();
let topZ = 20;
export function nextZ(): number {
	topZ += 1;
	return topZ;
}
let titles = 0;
export function mountWindows(mount: HTMLElement, roomID: string): void {
	table = mount;
	room = roomID;
	const template = document.querySelector("[data-window-template]");
	shell = template instanceof HTMLTemplateElement ? template : null;
	if (!shell) {
		return;
	}
	document.addEventListener("click", (e) => {
		if (!(e.target instanceof Element)) {
			return;
		}
		const trigger = e.target.closest("[data-window]");
		if (!(trigger instanceof HTMLElement)) {
			return;
		}
		const { window: id, windowUrl: url, windowTitle: title } = trigger.dataset;
		if (!id || !url || !title) {
			console.error("a window trigger is missing an id, a url or a title", trigger.dataset);
			return;
		}
		openWindow({
			id,
			url,
			title,
			width: size(trigger.dataset.windowWidth),
			height: size(trigger.dataset.windowHeight),
		});
	});
	let pending: number | undefined;
	window.addEventListener("resize", () => {
		window.clearTimeout(pending);
		pending = window.setTimeout(() => {
			for (const open of windows.values()) {
				open.reclamp();
			}
		}, RECLAMP_DELAY);
	});
	window.addEventListener(WINDOW_CLOSE, (e) => {
		const id = (e as CustomEvent<{ id?: string }>).detail?.id;
		if (id) {
			windows.get(id)?.close();
		}
	});
	window.addEventListener(WINDOW_RETITLE, (e) => {
		const detail = (e as CustomEvent<{ id?: string; title?: string }>).detail;
		if (detail?.id && detail.title) {
			windows.get(detail.id)?.retitle(detail.title);
		}
	});
	for (const spec of restored()) {
		openWindow(spec);
	}
}
export function openWindows(): string[] {
	return [...windows.keys()];
}
export function openWindow(spec: WindowSpec): void {
	if (!spec.url.startsWith(FRAGMENT_PREFIX)) {
		console.error(`a window needs a ${FRAGMENT_PREFIX} url, refusing:`, spec.url);
		return;
	}
	if (!table || !shell) {
		return;
	}
	const open = windows.get(spec.id);
	if (open) {
		open.reveal();
		return;
	}
	try {
		windows.set(spec.id, new RoomWindow(spec, table, shell));
	} catch (err) {
		console.error(`the ${spec.id} window could not be opened`, err);
		windows.delete(spec.id);
	}
	remember();
}
class RoomWindow {
	readonly id: string;
	readonly url: string;
	private title: string;
	private readonly el: HTMLElement;
	private readonly bar: HTMLElement;
	private readonly content: HTMLElement;
	private readonly panes: Map<string, HTMLElement>;
	private x = 0;
	private y = 0;
	private w = DEFAULT_WIDTH;
	private h = DEFAULT_HEIGHT;
	private minimized = false;
	private restoreTo: Geometry | null = null;
	private request: unknown = null;
	constructor(spec: WindowSpec, into: HTMLElement, from: HTMLTemplateElement) {
		this.id = spec.id;
		this.title = spec.title;
		this.url = spec.url;
		const root = from.content.firstElementChild?.cloneNode(true);
		if (!(root instanceof HTMLElement)) {
			throw new Error("the window template has no element in it");
		}
		this.el = root;
		this.bar = must(root, "[data-window-bar]");
		this.content = must(root, '[data-window-state="content"]');
		this.panes = new Map([
			["loading", must(root, '[data-window-state="loading"]')],
			["error", must(root, '[data-window-state="error"]')],
			["content", this.content],
		]);
		const heading = must(root, "[data-window-title]");
		heading.textContent = spec.title;
		titles += 1;
		heading.id = `window-title-${titles}`;
		root.setAttribute("aria-labelledby", heading.id);
		const saved = geometry(spec.id);
		this.w = saved?.w ?? spec.width ?? DEFAULT_WIDTH;
		this.h = saved?.h ?? spec.height ?? DEFAULT_HEIGHT;
		this.x = saved?.x ?? 16;
		this.y = saved?.y ?? 16;
		this.wire();
		into.append(root);
		this.reclamp();
		this.raise();
		this.load();
	}
	private wire(): void {
		this.el.addEventListener("pointerdown", () => this.raise());
		this.bar.addEventListener("pointerdown", (e) => this.startDrag(e));
		for (const handle of this.el.querySelectorAll("[data-window-resize]")) {
			if (!(handle instanceof HTMLElement)) {
				continue;
			}
			const axis = handle.dataset.windowResize ?? "both";
			handle.addEventListener("pointerdown", (e) => this.startResize(e, axis));
		}
		this.el.addEventListener("click", (e) => {
			if (!(e.target instanceof Element)) {
				return;
			}
			const action = e.target.closest("[data-window-action]");
			if (!(action instanceof HTMLElement) || this.content.contains(action)) {
				return;
			}
			switch (action.dataset.windowAction) {
				case "minimize":
					this.collapse(!this.minimized);
					break;
				case "maximize":
					this.toggleMaximize();
					break;
				case "close":
					this.close();
					break;
				case "retry":
					this.load();
					break;
			}
		});
		this.el.addEventListener("htmx:before:request", (e) => {
			if (e.target !== this.el) {
				return;
			}
			this.request = (e as CustomEvent<RequestDetail>).detail.ctx;
		});
		this.el.addEventListener("htmx:before:swap", (e) => {
			if (e.target === this.el && (e as CustomEvent<RequestDetail>).detail.ctx !== this.request) {
				e.preventDefault();
			}
		});
		this.el.addEventListener("htmx:finally:request", (e) => {
			const detail = (e as CustomEvent<RequestDetail>).detail;
			if (e.target !== this.el || detail.ctx !== this.request) {
				return;
			}
			const status = detail.ctx.response?.status;
			this.show(status !== undefined && status < 400 ? "content" : "error");
		});
	}
	private load(): void {
		this.show("loading");
		this.content.replaceChildren();
		this.request = null;
		if (typeof htmx === "undefined") {
			this.show("error");
			return;
		}
		htmx.ajax("GET", this.url, { target: this.content, source: this.el });
	}
	private show(state: string): void {
		for (const [name, pane] of this.panes) {
			pane.hidden = name !== state;
		}
	}
	reveal(): void {
		this.collapse(false);
		this.raise();
	}
	raise(): void {
		this.el.style.zIndex = String(nextZ());
	}
	private paint(): void {
		this.el.style.transform = `translate(${this.x}px, ${this.y}px)`;
		this.el.style.width = `${this.w}px`;
		this.el.style.height = `${this.height()}px`;
	}
	private height(): number {
		return this.minimized ? this.bar.offsetHeight : this.h;
	}
	private moveTo(x: number, y: number): void {
		const area = bounds();
		const tall = this.height();
		this.x = clamp(snap(x, this.w, this.lines("x")), 0, Math.max(0, area.w - this.w));
		this.y = clamp(snap(y, tall, this.lines("y")), 0, Math.max(0, area.h - tall));
		schedule(this);
	}
	private resizeTo(w: number | null, h: number | null): void {
		const area = bounds();
		if (w !== null) {
			const right = nearest(this.x + w, this.lines("x"));
			this.w = clamp(right - this.x, MIN_WIDTH, Math.max(MIN_WIDTH, area.w - this.x));
		}
		if (h !== null) {
			const bottom = nearest(this.y + h, this.lines("y"));
			this.h = clamp(bottom - this.y, MIN_HEIGHT, Math.max(MIN_HEIGHT, area.h - this.y));
		}
		schedule(this);
	}
	private lines(axis: "x" | "y"): number[] {
		const area = bounds();
		const out = [0, axis === "x" ? area.w : area.h];
		for (const other of windows.values()) {
			if (other === this) {
				continue;
			}
			if (axis === "x") {
				out.push(other.x, other.x + other.w);
			} else {
				out.push(other.y, other.y + other.height());
			}
		}
		return out;
	}
	reclamp(): void {
		const area = bounds();
		if (this.restoreTo) {
			this.x = 0;
			this.y = 0;
			this.w = area.w;
			this.h = area.h;
			this.commit();
			return;
		}
		this.w = clamp(this.w, MIN_WIDTH, Math.max(MIN_WIDTH, area.w));
		this.h = clamp(this.h, MIN_HEIGHT, Math.max(MIN_HEIGHT, area.h));
		this.x = clamp(this.x, 0, Math.max(0, area.w - this.w));
		this.y = clamp(this.y, 0, Math.max(0, area.h - this.height()));
		this.commit();
	}
	private collapse(minimized: boolean): void {
		this.minimized = minimized;
		this.el.querySelector("[data-window-body]")?.toggleAttribute("hidden", minimized);
		this.reclamp();
	}
	private toggleMaximize(): void {
		const area = bounds();
		if (this.restoreTo) {
			const back = this.restoreTo;
			this.restoreTo = null;
			this.x = back.x;
			this.y = back.y;
			this.w = back.w;
			this.h = back.h;
		} else {
			this.restoreTo = { x: this.x, y: this.y, w: this.w, h: this.h };
			this.x = 0;
			this.y = 0;
			this.w = area.w;
			this.h = area.h;
		}
		this.icon("maximize", this.restoreTo !== null);
		this.icon("restore", this.restoreTo === null);
		this.reclamp();
	}
	private icon(name: string, hide: boolean): void {
		const icon = this.el.querySelector(`[data-window-icon="${name}"]`);
		if (icon instanceof HTMLElement) {
			icon.hidden = hide;
		}
	}
	close(): void {
		this.save();
		this.el.remove();
		windows.delete(this.id);
		remember();
	}
	save(): void {
		store(`window:${this.id}`, this.restoreTo ?? { x: this.x, y: this.y, w: this.w, h: this.h });
	}
	retitle(title: string): void {
		if (title === "" || title === this.title) {
			return;
		}
		this.title = title;
		must(this.el, "[data-window-title]").textContent = title;
		remember();
	}
	spec(): WindowSpec {
		return { id: this.id, title: this.title, url: this.url };
	}
	commit(): void {
		this.paint();
	}
	private startDrag(e: PointerEvent): void {
		if (e.target instanceof Element && e.target.closest("button")) {
			return;
		}
		if (this.restoreTo) {
			return;
		}
		const offsetX = e.clientX - this.x;
		const offsetY = e.clientY - this.y;
		this.drag(this.bar, e, (ev) => this.moveTo(ev.clientX - offsetX, ev.clientY - offsetY));
	}
	private startResize(e: PointerEvent, axis: string): void {
		if (this.restoreTo || this.minimized) {
			return;
		}
		e.preventDefault();
		const offsetW = e.clientX - this.w;
		const offsetH = e.clientY - this.h;
		this.drag(e.currentTarget as HTMLElement, e, (ev) => {
			this.resizeTo(
				axis === "y" ? null : ev.clientX - offsetW,
				axis === "x" ? null : ev.clientY - offsetH,
			);
		});
	}
	private drag(on: HTMLElement, e: PointerEvent, step: (ev: PointerEvent) => void): void {
		this.raise();
		on.setPointerCapture(e.pointerId);
		const move = (ev: PointerEvent) => step(ev);
		const stop = () => {
			on.removeEventListener("pointermove", move);
			on.removeEventListener("pointerup", stop);
			on.removeEventListener("pointercancel", stop);
			this.save();
		};
		on.addEventListener("pointermove", move);
		on.addEventListener("pointerup", stop);
		on.addEventListener("pointercancel", stop);
	}
}
const moved = new Set<RoomWindow>();
let frame = 0;
function schedule(win: RoomWindow): void {
	moved.add(win);
	if (frame !== 0) {
		return;
	}
	frame = requestAnimationFrame(() => {
		frame = 0;
		for (const win of moved) {
			win.commit();
		}
		moved.clear();
	});
}
function bounds(): { w: number; h: number } {
	return { w: table?.clientWidth ?? 0, h: table?.clientHeight ?? 0 };
}
function clamp(value: number, low: number, high: number): number {
	return Math.min(Math.max(value, low), high);
}
function snap(start: number, size: number, lines: number[]): number {
	let best = start;
	let closest = SNAP;
	for (const line of lines) {
		const lead = Math.abs(start - line);
		if (lead < closest) {
			closest = lead;
			best = line;
		}
		const trail = Math.abs(start + size - line);
		if (trail < closest) {
			closest = trail;
			best = line - size;
		}
	}
	return best;
}
function nearest(value: number, lines: number[]): number {
	let best = value;
	let closest = SNAP;
	for (const line of lines) {
		const distance = Math.abs(value - line);
		if (distance < closest) {
			closest = distance;
			best = line;
		}
	}
	return best;
}
function must(root: HTMLElement, selector: string): HTMLElement {
	const found = root.querySelector(selector);
	if (!(found instanceof HTMLElement)) {
		throw new Error(`the window template has no ${selector}`);
	}
	return found;
}
function size(value: string | undefined): number | undefined {
	const parsed = Number.parseInt(value ?? "", 10);
	return Number.isFinite(parsed) ? parsed : undefined;
}
function geometry(id: string): Geometry | null {
	const saved = read<Partial<Geometry>>(`window:${id}`);
	if (!saved || typeof saved.x !== "number" || typeof saved.y !== "number") {
		return null;
	}
	return {
		x: saved.x,
		y: saved.y,
		w: typeof saved.w === "number" ? saved.w : DEFAULT_WIDTH,
		h: typeof saved.h === "number" ? saved.h : DEFAULT_HEIGHT,
	};
}
function remember(): void {
	const open: WindowSpec[] = [];
	for (const win of windows.values()) {
		open.push(win.spec());
	}
	store(`windows:${room}`, open);
}
function restored(): WindowSpec[] {
	const saved = read<WindowSpec[]>(`windows:${room}`);
	if (!Array.isArray(saved)) {
		return [];
	}
	return saved.filter(
		(spec): spec is WindowSpec =>
			typeof spec?.id === "string" &&
			typeof spec?.title === "string" &&
			typeof spec?.url === "string",
	);
}
