import { TRANSIENT_EVENTS, type Command, type Frame } from "./protocol.ts";
type Unsent<T> = T extends { cid: string } ? Omit<T, "cid"> : never;
export type Outgoing = Unsent<Command>;
export type Status = "connecting" | "open" | "closed" | "ended";
export type Dropped = "duplicate" | "gap" | "unparsed" | "stale" | "skipped";
export interface SocketHandlers {
	frame(frame: Frame): void;
	status(status: Status, detail: string): void;
}
export interface Monitor {
	sent(text: string): void;
	received(text: string, frame: Frame | null, dropped: Dropped | null): void;
	status(status: Status, detail: string): void;
}
export interface Impairment {
	latency: number;
	jitter: number;
	loss: number;
}
export interface SocketStats {
	open: boolean;
	sent: number;
	sentBytes: number;
	received: number;
	receivedBytes: number;
	duplicates: number;
	gaps: number;
	resyncs: number;
	connects: number;
	attempt: number;
	backoff: number;
	since: number;
	reason: string;
	skipping: number;
	lost: number;
	delayed: number;
}
const backoffFloor = 500;
const backoffCeiling = 15_000;
export const ENDED_REASONS: ReadonlySet<string> = new Set(["kicked", "left", "limit"]);
export class Socket {
	private readonly url: string;
	private readonly handlers: SocketHandlers;
	private monitor: Monitor | null = null;
	private ws: WebSocket | null = null;
	private timer: number | undefined;
	private attempt = 0;
	private cid = 0;
	private seq = 0;
	private version = "";
	private ended = false;
	private resyncing = false;
	private skipping = 0;
	private impairment: Impairment | null = null;
	private counts = {
		sent: 0,
		sentBytes: 0,
		received: 0,
		receivedBytes: 0,
		duplicates: 0,
		gaps: 0,
		resyncs: 0,
		connects: 0,
		backoff: 0,
		since: 0,
		reason: "",
		lost: 0,
		delayed: 0,
	};
	constructor(url: string, handlers: SocketHandlers) {
		this.url = url;
		this.handlers = handlers;
	}
	watch(monitor: Monitor | null): void {
		this.monitor = monitor;
	}
	stats(): SocketStats {
		return {
			open: this.ws !== null && this.ws.readyState === WebSocket.OPEN,
			attempt: this.attempt,
			skipping: this.skipping,
			...this.counts,
		};
	}
	skip(frames: number): void {
		this.skipping = Math.max(0, frames);
	}
	impair(impairment: Impairment | null): void {
		this.impairment = impairment;
	}
	impaired(): Impairment | null {
		return this.impairment;
	}
	disconnect(): boolean {
		if (!this.ws) {
			return false;
		}
		this.ws.close(1000, "");
		return true;
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
		const ws = this.ws;
		if (!ws || ws.readyState !== WebSocket.OPEN) {
			return false;
		}
		this.counts.sent += 1;
		this.counts.sentBytes += text.length;
		this.monitor?.sent(text);
		const impairment = this.impairment;
		if (!impairment) {
			ws.send(text);
			return true;
		}
		if (Math.random() < impairment.loss) {
			this.counts.lost += 1;
			return true;
		}
		const wait = impairment.latency + Math.random() * impairment.jitter;
		if (wait <= 0) {
			ws.send(text);
			return true;
		}
		this.counts.delayed += 1;
		window.setTimeout(() => {
			if (ws.readyState === WebSocket.OPEN) {
				ws.send(text);
			}
		}, wait);
		return true;
	}
	sequence(): number {
		return this.seq;
	}
	build(): string {
		return this.version;
	}
	private report(status: Status, detail: string): void {
		this.monitor?.status(status, detail);
		this.handlers.status(status, detail);
	}
	private connect(): void {
		this.counts.connects += 1;
		this.report("connecting", "");
		const ws = new WebSocket(new URL(this.url, location.href));
		this.ws = ws;
		ws.addEventListener("open", () => {
			this.attempt = 0;
			this.counts.since = performance.now();
			this.report("open", "");
		});
		ws.addEventListener("message", (e) => {
			if (typeof e.data === "string") {
				this.receive(e.data);
			}
		});
		ws.addEventListener("close", (e) => {
			this.ws = null;
			this.counts.since = 0;
			this.counts.reason = e.reason;
			if (ENDED_REASONS.has(e.reason)) {
				this.ended = true;
			}
			if (this.ended) {
				this.report("ended", e.reason);
				return;
			}
			this.report("closed", e.reason);
			this.reconnect(this.backoff());
		});
	}
	private receive(text: string): void {
		this.counts.received += 1;
		this.counts.receivedBytes += text.length;
		let frame: Frame;
		try {
			frame = JSON.parse(text) as Frame;
		} catch {
			this.monitor?.received(text, null, "unparsed");
			return;
		}
		if (frame.type === "snapshot") {
			this.seq = frame.seq;
			this.resyncing = false;
			if (this.version === "") {
				this.version = frame.version;
			} else if (this.version !== frame.version) {
				this.monitor?.received(text, frame, "stale");
				location.reload();
				return;
			}
			this.deliver(text, frame);
			return;
		}
		if (frame.type === "room.closed") {
			this.ended = true;
			this.deliver(text, frame);
			return;
		}
		if (TRANSIENT_EVENTS.has(frame.type)) {
			this.deliver(text, frame);
			return;
		}
		if (this.skipping > 0) {
			this.skipping -= 1;
			this.monitor?.received(text, frame, "skipped");
			return;
		}
		if (frame.seq <= this.seq) {
			this.counts.duplicates += 1;
			this.monitor?.received(text, frame, "duplicate");
			return;
		}
		if (frame.seq > this.seq + 1) {
			this.counts.gaps += 1;
			this.monitor?.received(text, frame, "gap");
			this.resync();
			return;
		}
		this.seq = frame.seq;
		this.deliver(text, frame);
	}
	private deliver(text: string, frame: Frame): void {
		this.monitor?.received(text, frame, null);
		this.handlers.frame(frame);
	}
	resync(): void {
		if (this.resyncing) {
			return;
		}
		this.resyncing = true;
		this.counts.resyncs += 1;
		this.send({ type: "sync.request" });
	}
	private reconnect(delay: number): void {
		window.clearTimeout(this.timer);
		this.timer = window.setTimeout(() => this.connect(), delay);
	}
	private backoff(): number {
		const step = Math.min(backoffCeiling, backoffFloor * 2 ** this.attempt);
		this.attempt += 1;
		this.counts.backoff = step / 2 + Math.random() * (step / 2);
		return this.counts.backoff;
	}
}
