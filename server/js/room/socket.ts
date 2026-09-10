// The connection: connect, reconnect, notice a gap, and send.
//
// FRAMES ARRIVE IN THE ORDER THEY WERE SENT AND THIS CODE DEPENDS ON IT. A
// WebSocket is one TCP connection with no multiplexing, so the browser queues
// message events in receive order; there is no reorder buffer here because
// there is nothing to reorder. What CAN happen is a gap -- the server dropped a
// frame, or this is a different connection than the one the sequence came from
// -- and that is what the sequence check below is for.
//
// SO NOTHING IN THIS FILE MAY AWAIT BETWEEN RECEIVING AND REDUCING. An async
// handler fires in order and completes out of order, which looks exactly like
// transport reordering and is the bug people mistake for it. Parse, reduce,
// hand on; anything slower belongs behind the render loop's dirty flag.

import { TRANSIENT_EVENTS, type Command, type Event } from "./protocol.ts";

// Unsent is a command without the correlation id, because send assigns that.
// The conditional type is what distributes the omission over the union -- a
// plain Omit<Command, "cid"> would collapse twenty-odd interfaces into their
// common fields and lose the discriminant.
type Unsent<T> = T extends { cid: string } ? Omit<T, "cid"> : never;

export type Outgoing = Unsent<Command>;

// Status is what the debug panel and, later, a reconnecting indicator show.
export type Status = "connecting" | "open" | "closed" | "ended";

export interface SocketHandlers {
	// event is every frame, after the sequence check and in order. The socket
	// does not reduce: main wires that up, so the store and the effects layer
	// are visible in one place.
	event(event: Event): void;
	status(status: Status, detail: string): void;
}

const backoffFloor = 500;
const backoffCeiling = 15_000;

// ENDED_REASONS are the close reasons a client must not reconnect after. The
// GM removed this person; this person pressed Leave in another tab; or this
// tab is one past the number of connections one person may hold. Everything
// else -- a restart, a dropped connection, a laptop lid -- reconnects.
//
// THE WORDS ARE THE SERVER'S, in internal/hub/conn.go, and a Go test reads this
// file to hold the two lists together.
export const ENDED_REASONS: ReadonlySet<string> = new Set(["kicked", "left", "limit"]);

export class Socket {
	private readonly url: string;
	private readonly handlers: SocketHandlers;

	private ws: WebSocket | null = null;
	private timer: number | undefined;
	private attempt = 0;
	private cid = 0;

	// seq is the last counted frame this client applied. Transient frames carry
	// the current value without advancing it, so they are checked against
	// nothing -- see receive.
	private seq = 0;

	// version is the server build the first snapshot named. A later snapshot
	// naming a different one means this tab is running a bundle from before a
	// deploy, and the answer to that is a reload rather than speaking a
	// protocol it was not generated against.
	private version = "";

	// ended is set by the two things a client must not reconnect after: being
	// removed from the room, and the room closing. Everything else -- a
	// restart, a dropped connection, a laptop lid -- reconnects.
	private ended = false;

	// resyncing stops a run of gaps from sending a request per frame. One goes
	// out, and the snapshot that answers it clears the flag by resetting
	// everything.
	private resyncing = false;

	constructor(url: string, handlers: SocketHandlers) {
		this.url = url;
		this.handlers = handlers;
	}

	start(): void {
		this.connect();

		// A tab that was in the background while the wifi came back should not
		// wait out the rest of its backoff before anybody sees it again.
		document.addEventListener("visibilitychange", () => {
			if (document.visibilityState === "visible" && !this.ws && !this.ended) {
				this.reconnect(0);
			}
		});
	}

	// send stringifies one command and gives it a correlation id, so a refusal
	// can be matched to the request that caused it. It returns false when there
	// is no connection.
	//
	// NOTHING IS QUEUED WHILE OFFLINE, deliberately. The old client kept an
	// offline queue and replayed it after reconnecting, which put stale moves
	// on the table seconds after everybody had moved on. Every command here is
	// either idempotent by snapshot or transient, so dropping one and letting
	// the person do it again is both simpler and more correct.
	send(command: Outgoing): boolean {
		this.cid += 1;

		return this.raw(JSON.stringify({ ...command, cid: String(this.cid) }));
	}

	// raw is the debug panel's door: whatever was typed, sent verbatim.
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

			// 1008 with one of these reasons is a close a client must not
			// answer by coming back; see ENDED_REASONS.
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
			// A snapshot is where the sequence comes from rather than a step in
			// it. It is not numbered, so adopting it is how a gap is repaired.
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

		// A transient frame carries the current sequence without advancing it,
		// so it is delivered without a check. Checking it would drop every ping
		// that arrived between two state events.
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

	// resync asks for the whole room again and buffers nothing in the meantime.
	//
	// BUFFERING WOULD BE THE HARDER HALF OF A REPLAY LOG AND BUY NOTHING. The
	// snapshot that answers this replaces everything, so a frame held back
	// while it was in flight is a frame the snapshot already contains.
	//
	// IT IS PUBLIC FOR ONE CALLER: a refusal with the not_found code, which is
	// the server saying this client is holding something that is gone. The
	// snapshot is the honest answer to that, and it is the same answer a gap
	// gets.
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

	// backoff is exponential from half a second to fifteen, with jitter, which
	// is what keeps a table's worth of browsers from all arriving at a
	// restarting server in the same millisecond.
	private backoff(): number {
		const step = Math.min(backoffCeiling, backoffFloor * 2 ** this.attempt);
		this.attempt += 1;

		return step / 2 + Math.random() * (step / 2);
	}
}
