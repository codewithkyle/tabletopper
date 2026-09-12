import assert from "node:assert/strict";
import { test } from "node:test";
import type { Frame } from "./protocol.ts";
class FakeSocket {
	static readonly OPEN = 1;
	static last: FakeSocket | null = null;
	readyState = FakeSocket.OPEN;
	readonly sent: string[] = [];
	readonly url: URL | string;
	private readonly listeners = new Map<string, ((e: unknown) => void)[]>();
	constructor(url: URL | string) {
		this.url = url;
		FakeSocket.last = this;
	}
	addEventListener(name: string, listener: (e: unknown) => void): void {
		this.listeners.set(name, [...(this.listeners.get(name) ?? []), listener]);
	}
	send(text: string): void {
		this.sent.push(text);
	}
	close(_code?: number, reason?: string): void {
		this.fire("close", { reason: reason ?? "" });
	}
	fire(name: string, e: unknown): void {
		for (const listener of this.listeners.get(name) ?? []) {
			listener(e);
		}
	}
	deliver(frame: unknown): void {
		this.fire("message", { data: JSON.stringify(frame) });
	}
}
Object.assign(globalThis, {
	WebSocket: FakeSocket,
	document: { addEventListener: () => {}, visibilityState: "visible" },
	location: { href: "https://table.test/rooms/01ROOM" },
	window: { setTimeout: () => 0, clearTimeout: () => {} },
});
const { Socket } = await import("./socket.ts");
function open(): { socket: InstanceType<typeof Socket>; seen: Frame[]; ws: FakeSocket } {
	const seen: Frame[] = [];
	const socket = new Socket("/socket/room/01ROOM", {
		frame: (frame) => seen.push(frame),
		status: () => {},
	});
	socket.start();
	const ws = FakeSocket.last;
	assert.ok(ws, "the socket did not open a connection");
	ws.fire("open", {});
	ws.deliver(snapshot(7));
	seen.length = 0;
	return { socket, seen, ws };
}
function snapshot(seq: number): Frame {
	return {
		type: "snapshot",
		seq,
		state: { seq } as never,
		you: { id: "01ME", role: "gm" },
		version: "test-build",
		now: 0,
	};
}
function changes(seq: number): Frame {
	return { type: "changes", seq, events: [{ type: "pawns.removed", ids: ["01GONE"] }] };
}
test("a snapshot sets the sequence the frames after it are measured against", () => {
	const { socket } = open();
	assert.equal(socket.sequence(), 7);
});
test("the next frame in line is delivered and moves the sequence on", () => {
	const { socket, seen, ws } = open();
	ws.deliver(changes(8));
	assert.deepEqual(seen.map((f) => f.type), ["changes"]);
	assert.equal(socket.sequence(), 8);
});
test("a gap asks for the room again and delivers nothing", () => {
	const { socket, seen, ws } = open();
	ws.deliver(changes(10));
	assert.deepEqual(seen, [], "a frame past the gap was applied to a stale room");
	assert.equal(socket.sequence(), 7, "the gap moved the sequence on");
	assert.deepEqual(
		ws.sent.map((text) => (JSON.parse(text) as { type: string }).type),
		["sync.request"],
	);
});
test("one gap asks once however many frames arrive behind it", () => {
	const { ws } = open();
	ws.deliver(changes(10));
	ws.deliver(changes(11));
	assert.equal(ws.sent.length, 1);
});
test("a frame already seen is dropped", () => {
	const { socket, seen, ws } = open();
	ws.deliver(changes(7));
	ws.deliver(changes(6));
	assert.deepEqual(seen, []);
	assert.equal(socket.sequence(), 7);
});
test("a transient is delivered and leaves the sequence where it was", () => {
	const { socket, seen, ws } = open();
	ws.deliver({ type: "pinged", seq: 7, layer: "01LAYER", x: 1, y: 1 });
	ws.deliver({ type: "pawn.dragging", seq: 7, pawns: [] });
	ws.deliver({ type: "error", seq: 7, cid: "1", code: "invalid", heading: "No", message: "No." });
	assert.deepEqual(seen.map((f) => f.type), ["pinged", "pawn.dragging", "error"]);
	assert.equal(socket.sequence(), 7);
	ws.deliver(changes(8));
	assert.equal(socket.sequence(), 8, "a transient left a gap behind it");
});
test("a snapshot after a resync sets the sequence wherever the server is", () => {
	const { socket, seen, ws } = open();
	ws.deliver(changes(10));
	ws.deliver(snapshot(12));
	assert.deepEqual(seen.map((f) => f.type), ["snapshot"]);
	assert.equal(socket.sequence(), 12);
	ws.deliver(changes(13));
	assert.equal(socket.sequence(), 13);
});
function watched(): {
	socket: InstanceType<typeof Socket>;
	ws: FakeSocket;
	seen: Frame[];
	log: string[];
} {
	const { socket, seen, ws } = open();
	const log: string[] = [];
	socket.watch({
		sent: (text) => log.push(`> ${(JSON.parse(text) as { type: string }).type}`),
		received: (_text, frame, dropped) => log.push(`< ${frame?.type ?? "unreadable"}${dropped ? ` ${dropped}` : ""}`),
		status: (status) => log.push(`- ${status}`),
	});
	return { socket, ws, seen, log };
}
test("a monitor sees the frames the socket drops, which nothing else does", () => {
	const { ws, seen, log } = watched();
	ws.deliver(changes(7));
	ws.deliver(changes(20));
	assert.deepEqual(seen, [], "neither frame should reach the room");
	assert.deepEqual(log, ["< changes duplicate", "< changes gap", "> sync.request"]);
});
test("a monitor sees a frame that would not parse", () => {
	const { ws, log } = watched();
	ws.fire("message", { data: "{not json" });
	assert.deepEqual(log, ["< unreadable unparsed"]);
});
test("a monitor sees a snapshot before the room has reduced it", () => {
	const { ws, log, seen } = watched();
	ws.deliver(snapshot(30));
	assert.equal(log.indexOf("< snapshot"), 0, "the monitor is told first");
	assert.equal(seen.length, 1, "and the room is told after");
});
test("the socket counts what it sent, received and threw away", () => {
	const { socket, ws } = watched();
	ws.deliver(changes(7));
	ws.deliver(changes(20));
	ws.deliver(changes(8));
	const stats = socket.stats();
	assert.equal(stats.duplicates, 1);
	assert.equal(stats.gaps, 1);
	assert.equal(stats.resyncs, 1);
	assert.equal(stats.sent, 1, "the resync is the only thing sent");
	assert.ok(stats.received >= 4, "the opening snapshot counts too");
	assert.ok(stats.receivedBytes > 0);
	assert.ok(stats.sentBytes > 0);
	assert.equal(stats.connects, 1);
	assert.equal(stats.open, true);
});
test("a closed socket reports why and stops counting uptime", () => {
	const { socket, ws } = watched();
	assert.ok(socket.stats().since > 0, "an open socket has been up for some time");
	ws.fire("close", { reason: "slow" });
	const stats = socket.stats();
	assert.equal(stats.open, false);
	assert.equal(stats.reason, "slow");
	assert.equal(stats.since, 0);
	assert.ok(stats.backoff > 0, "a reconnect was scheduled");
});
test("dropping the monitor leaves the socket delivering as before", () => {
	const { socket, ws, seen, log } = watched();
	socket.watch(null);
	ws.deliver(changes(8));
	assert.deepEqual(log, []);
	assert.deepEqual(seen.map((f) => f.type), ["changes"]);
});
test("skipping frames makes the socket miss its place and ask for the room again", () => {
	const { socket, seen, ws, log } = watched();
	socket.skip(2);
	ws.deliver(changes(8));
	ws.deliver(changes(9));
	assert.deepEqual(seen, [], "the skipped frames must not reach the room");
	assert.equal(socket.sequence(), 7, "a skipped frame does not move the sequence");
	assert.equal(socket.stats().skipping, 0, "both skips were spent");
	ws.deliver(changes(10));
	assert.deepEqual(log.at(-1), "> sync.request", "the gap the skips left was not noticed");
	assert.equal(socket.stats().gaps, 1);
});
test("a skip leaves a transient alone, because a transient carries no place in line", () => {
	const { socket, seen, ws } = watched();
	socket.skip(1);
	ws.deliver({ type: "pinged", seq: 7, layer: "01LAYER", x: 1, y: 1 });
	assert.deepEqual(seen.map((f) => f.type), ["pinged"]);
	assert.equal(socket.stats().skipping, 1, "the skip is still waiting for a sequenced frame");
});
test("dropping the connection closes it and schedules a reconnect", () => {
	const { socket, ws } = watched();
	assert.equal(socket.disconnect(), true);
	assert.equal(ws.sent.length, 0, "dropping the connection sends nothing");
	assert.equal(socket.stats().open, false);
	assert.ok(socket.stats().backoff > 0);
	assert.equal(socket.disconnect(), false, "there is nothing left to drop");
});
test("impairment drops the share of commands it is asked to and counts them", () => {
	const { socket, ws } = watched();
	const real = Math.random;
	Math.random = () => 0;
	try {
		socket.impair({ latency: 0, jitter: 0, loss: 1 });
		assert.equal(socket.raw('{"type":"ping","cid":"1"}'), true, "the caller is told it went");
		assert.equal(ws.sent.length, 0, "but nothing reached the wire");
		assert.equal(socket.stats().lost, 1);
		assert.equal(socket.stats().sent, 1, "a dropped command is still one the client sent");
	} finally {
		Math.random = real;
	}
});
test("impairment with no latency and no loss still sends immediately", () => {
	const { socket, ws } = watched();
	socket.impair({ latency: 0, jitter: 0, loss: 0 });
	socket.raw('{"type":"ping","cid":"1"}');
	assert.equal(ws.sent.length, 1);
	assert.equal(socket.stats().delayed, 0);
});
test("clearing impairment puts the socket back on the wire", () => {
	const { socket, ws } = watched();
	socket.impair({ latency: 0, jitter: 0, loss: 1 });
	socket.impair(null);
	assert.equal(socket.impaired(), null);
	socket.raw('{"type":"ping","cid":"1"}');
	assert.equal(ws.sent.length, 1);
});
