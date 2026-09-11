import { TRANSIENT_EVENTS, type Command, type Event } from "./protocol.ts";
type Unsent<T> = T extends { cid: string } ? Omit<T, "cid"> : never;
export type Outgoing = Unsent<Command>;
export type Status = "connecting" | "open" | "closed" | "ended";
export interface SocketHandlers {
	event(event: Event): void;
	status(status: Status, detail: string): void;
}
const backoffFloor = 500;
const backoffCeiling = 15_000;
export const ENDED_REASONS: ReadonlySet<string> = new Set(["kicked", "left", "limit"]);
export class Socket {
	private readonly url: string;
	private readonly handlers: SocketHandlers;
	private ws: WebSocket | null = null;
	private timer: number | undefined;
	private attempt = 0;
	private cid = 0;
	private seq = 0;
	private version = "";
	private ended = false;
	private resyncing = false;
	constructor(url: string, handlers: SocketHandlers) {
		this.url = url;
		this.handlers = handlers;
	}
	start(): void {
		this.connect();
		document.addEventListener("visibilitychange", () => {
			if (document.visibilityState === "visible" && !this.ws && !this.ended) {
				this.reconnect(0);
			}
		});
	}
	send(command: Outgoing): boolean {
		this.cid += 1;
		return this.raw(JSON.stringify({ ...command, cid: String(this.cid) }));
	}
	raw(text: string): boolean {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
			return false;
		}
		this.ws.send(text);
		return true;
	}
	sequence(): number {
		return this.seq;
	}
	build(): string {
		return this.version;
	}
	private connect(): void {
		this.handlers.status("connecting", "");
		const ws = new WebSocket(new URL(this.url, location.href));
		this.ws = ws;
		ws.addEventListener("open", () => {
			this.attempt = 0;
			this.handlers.status("open", "");
		});
		ws.addEventListener("message", (e) => {
			if (typeof e.data === "string") {
				this.receive(e.data);
			}
		});
		ws.addEventListener("close", (e) => {
			this.ws = null;
			if (ENDED_REASONS.has(e.reason)) {
				this.ended = true;
			}
			if (this.ended) {
				this.handlers.status("ended", e.reason);
				return;
			}
			this.handlers.status("closed", e.reason);
			this.reconnect(this.backoff());
		});
	}
	private receive(text: string): void {
		let event: Event;
		try {
			event = JSON.parse(text) as Event;
		} catch {
			return;
		}
		if (event.type === "snapshot") {
			this.seq = event.seq;
			this.resyncing = false;
			if (this.version === "") {
				this.version = event.version;
			} else if (this.version !== event.version) {
				location.reload();
				return;
			}
			this.handlers.event(event);
			return;
		}
		if (event.type === "room.closed") {
			this.ended = true;
			this.handlers.event(event);
			return;
		}
		if (TRANSIENT_EVENTS.has(event.type)) {
			this.handlers.event(event);
			return;
		}
		if (event.seq <= this.seq) {
			return;
		}
		if (event.seq > this.seq + 1) {
			this.resync();
			return;
		}
		this.seq = event.seq;
		this.handlers.event(event);
	}
	resync(): void {
		if (this.resyncing) {
			return;
		}
		this.resyncing = true;
		this.send({ type: "sync.request" });
	}
	private reconnect(delay: number): void {
		window.clearTimeout(this.timer);
		this.timer = window.setTimeout(() => this.connect(), delay);
	}
	private backoff(): number {
		const step = Math.min(backoffCeiling, backoffFloor * 2 ** this.attempt);
		this.attempt += 1;
		return step / 2 + Math.random() * (step / 2);
	}
}
