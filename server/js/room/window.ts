// FLOATING WINDOWS OVER THE TABLE: the player list, the monster manual, a stat
// block, the layer and grid settings. Several at once, moved and resized and
// tucked into corners, remembered between visits.
//
// A WINDOW IS NOT A MODAL AND THE DIFFERENCE IS THE WHOLE DESIGN. A modal is
// one at a time, blocks the page, sits in the middle and is dismissed; a window
// blocks nothing, sits where it was put, and several are open while the GM
// works. The app's three <dialog> modals stay exactly as they are -- this is a
// different mechanism, not a fourth one of those.
//
// A WINDOW HOLDS A FRAGMENT URL AND NO VIEW CODE AT ALL. That is what makes it
// worth having: htmx processes hx-* in a response it swapped, so any
// /fragment/ route in the app becomes a window without a line of server change,
// and a fragment that refetches itself on a socket event goes on doing that
// inside one. The old client's windows each held a lit-html template and owned
// their content's lifecycle, which is why every window there was a bespoke
// component.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/js is not a Tailwind source, so
// a class named here would never be emitted. The chrome is a <template> in
// room.templ and everything below sets text, [hidden], and inline geometry.

const SNAP = 10;
const MIN_WIDTH = 200;
const MIN_HEIGHT = 120;
const DEFAULT_WIDTH = 280;
const DEFAULT_HEIGHT = 320;
const RECLAMP_DELAY = 150;

// Every window names a /fragment/ route and this checks rather than trusting
// the caller, for the reason the content modal checks: a URL that arrives as
// data can name a page, and a whole <html> document swapped into a window reads
// as a styling bug rather than a wrong URL. Comparing against a leading slash
// rules out an absolute or protocol-relative URL for free.
const FRAGMENT_PREFIX = "/fragment/";

// htmx is a global from base.templ, loaded before any module runs. Only the one
// call is used, and it is what gives a window the app's ordinary swap
// behaviour: hx-* inside the response is processed on arrival, so there is no
// htmx.process() here and there must not be one.
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

// The htmx request events carry the same ctx object for every event of one
// request, which is how a window tells its own content load apart from the
// requests that content makes once it has landed.
interface RequestDetail {
	ctx: { response?: { status?: number } };
}

let table: HTMLElement | null = null;
let shell: HTMLTemplateElement | null = null;
let room = "";

const windows = new Map<string, RoomWindow>();

// topZ is the stacking order among windows, and it is local: the table is its
// own stacking context, so these compete with the tool pill inside it and never
// with the menu bar above it.
let topZ = 20;

let titles = 0;

// mountWindows wires the room page up. It runs whether or not the room has a
// socket -- a closed room still has a player list to read.
export function mountWindows(mount: HTMLElement, roomID: string): void {
	table = mount;
	room = roomID;

	const template = document.querySelector("[data-window-template]");
	shell = template instanceof HTMLTemplateElement ? template : null;
	if (!shell) {
		return;
	}

	// A control asks for a window declaratively, the way one asks for the
	// content modal:
	//
	//   <button data-window="players" data-window-url="/fragment/room/members?room=..."
	//           data-window-title="Players">
	//
	// Delegated on the document, because a trigger can itself arrive in a swap.
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

	// A window that ran off the bottom of a smaller viewport is a window that
	// cannot be reached, so every one of them is pulled back inside. Debounced,
	// because a drag of the browser's own corner fires this continuously.
	let pending: number | undefined;
	window.addEventListener("resize", () => {
		window.clearTimeout(pending);
		pending = window.setTimeout(() => {
			for (const open of windows.values()) {
				open.reclamp();
			}
		}, RECLAMP_DELAY);
	});

	// WHAT WAS OPEN COMES BACK. A reload -- including the one the client gives
	// itself when the server build changes -- should not cost the GM the layout
	// they arranged.
	for (const spec of restored()) {
		openWindow(spec);
	}
}

// openWindow shows a window, or brings the one that is already open forward.
// Clicking a menu item for a window that is behind something, or collapsed to
// its title bar, means "show me that" rather than nothing.
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

	// ONE BAD WINDOW MUST NOT TAKE THE PAGE WITH IT. This runs during startup
	// for everything that was open last time, and it runs before the socket is
	// wired -- so a window that throws on a template this build no longer has
	// would otherwise cost the room its connection as well as its layout.
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
	readonly title: string;
	readonly url: string;

	private readonly el: HTMLElement;
	private readonly bar: HTMLElement;
	private readonly content: HTMLElement;
	private readonly panes: Map<string, HTMLElement>;

	private x = 0;
	private y = 0;
	private w = DEFAULT_WIDTH;
	private h = DEFAULT_HEIGHT;

	private minimized = false;

	// restoreTo holds the windowed geometry while the window is maximized, so a
	// non-null value IS "maximized" -- there is no second flag to keep in step
	// with it.
	private restoreTo: Geometry | null = null;

	// request is the ctx of the fetch whose content this window is expecting.
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
			// The chrome's own controls only. A fragment is free to use the same
			// attribute for something of its own without reaching this switch.
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

		// A load from an earlier Retry that is still in flight. Let it finish so
		// htmx settles its own bookkeeping, but do not let it paint over what is
		// on screen now.
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

			// A missing response means the fetch threw. A 4xx or 5xx arrives
			// here having swapped nothing, because the noSwap config in
			// base.templ covers both ranges -- without this the spinner would
			// run until the next navigation.
			const status = detail.ctx.response?.status;
			this.show(status !== undefined && status < 400 ? "content" : "error");
		});
	}

	private load(): void {
		this.show("loading");
		this.content.replaceChildren();
		this.request = null;

		// htmx is a global from base.templ and the document order there puts it
		// ahead of every module. If that ever stops being true the window says
		// so and offers Retry, rather than throwing out of a constructor.
		if (typeof htmx === "undefined") {
			this.show("error");

			return;
		}

		// Naming the window as the source puts every event for this fetch on the
		// window itself, which is how the listeners above tell it apart from the
		// requests the content makes once it has landed -- a live panel
		// refetching itself bubbles through here too, with its own target.
		htmx.ajax("GET", this.url, { target: this.content, source: this.el });
	}

	private show(state: string): void {
		for (const [name, pane] of this.panes) {
			pane.hidden = name !== state;
		}
	}

	// reveal is what a second click on the menu item does: bring it forward, and
	// open it back up if it was collapsed.
	reveal(): void {
		this.collapse(false);
		this.raise();
	}

	raise(): void {
		topZ += 1;
		this.el.style.zIndex = String(topZ);
	}

	private paint(): void {
		this.el.style.transform = `translate(${this.x}px, ${this.y}px)`;
		this.el.style.width = `${this.w}px`;
		this.el.style.height = `${this.height()}px`;
	}

	// height is what the window occupies on screen, which is its title bar alone
	// while it is collapsed. Every clamp and every snap line reads this rather
	// than h, so a minimized window can still be dragged to the bottom edge and
	// still snaps against its neighbours.
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

	// null on an axis is "this handle does not touch that one", which is what
	// keeps the east handle from nudging the height by a pixel.
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

	// lines is everything worth snapping to on one axis: the table's own two
	// edges and every other window's. Snapping a window's leading edge to
	// another's trailing edge is what stacks two of them flush; snapping leading
	// to leading is what lines a column of them up.
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

	// reclamp pulls a window back inside the table, which is what a browser
	// resize and a first open both need. A maximized one simply fills the new
	// size.
	reclamp(): void {
		const area = bounds();

		// PAINTED HERE AND NOT SCHEDULED. Every caller is a one-off -- opening,
		// collapsing, maximizing, a browser resize -- and a window that waited a
		// frame for its first geometry would be drawn once at its natural size
		// in the corner before it moved.
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

	// collapse hides the body and leaves the title bar, which is the corner-
	// tucking move: a window you want to know is there and do not want to read.
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

	// save writes the windowed geometry, never the maximized one -- restoring a
	// window to the size of somebody else's screen is not restoring it.
	save(): void {
		store(`window:${this.id}`, this.restoreTo ?? { x: this.x, y: this.y, w: this.w, h: this.h });
	}

	spec(): WindowSpec {
		return { id: this.id, title: this.title, url: this.url };
	}

	commit(): void {
		this.paint();
	}

	private startDrag(e: PointerEvent): void {
		// The title bar is also where the three controls are, and a click on one
		// of those is not a drag.
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

	// drag is the pointer bookkeeping both gestures share. Capture is what keeps
	// a fast pointer that has left the element still driving it, and one code
	// path covers mouse, pen and touch -- the old client had two.
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

// THE FRAME LOOP, and it is here for the reason the renderer will have one: a
// pointermove fires far more often than the screen refreshes, and writing a
// transform per event is layout work the browser throws away. Handlers record
// where the window should be; one animation frame paints every window that
// moved.
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

// bounds is the table, which is what a window may not leave. Measuring the
// element rather than the viewport means the menu bar is accounted for without
// this file knowing how tall it is.
function bounds(): { w: number; h: number } {
	return { w: table?.clientWidth ?? 0, h: table?.clientHeight ?? 0 };
}

function clamp(value: number, low: number, high: number): number {
	return Math.min(Math.max(value, low), high);
}

// snap moves a whole window so that whichever of its two edges is closest to a
// line lands on it.
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

// nearest is snap for one edge, which is what a resize moves.
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

// WHERE A WINDOW SITS IS ONE VIEWER'S CONVENIENCE AND NOBODY ELSE'S BUSINESS,
// so it is localStorage rather than anything the server hears about. Every
// accessor is guarded: a private window, cleared site data or a browser set to
// refuse storage all throw here rather than returning nothing.
function store(key: string, value: unknown): void {
	try {
		localStorage.setItem(`tabletopper:${key}`, JSON.stringify(value));
	} catch {
		// A layout that is not remembered is not a reason to stop working.
	}
}

function read<T>(key: string): T | null {
	try {
		const raw = localStorage.getItem(`tabletopper:${key}`);

		return raw === null ? null : (JSON.parse(raw) as T);
	} catch {
		return null;
	}
}

// Geometry is keyed by the window rather than by the room, so the player list is
// where you left it at every table.
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

// Which windows are open IS keyed by the room, because it is a fact about this
// table rather than about this person's habits.
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
