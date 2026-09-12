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
