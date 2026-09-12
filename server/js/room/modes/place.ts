import type { Ghostable } from "../model/overlay.ts";
import type { Grid, PawnKind, Size } from "../protocol.ts";
import type { Outgoing } from "../socket.ts";
import type { Point } from "../model/types.ts";
import type { Tool } from "../render/input.ts";
import { blankDrawn, ghostOf } from "../model/overlay.ts";
import { snapTo } from "../model/shape.ts";
export interface Armed {
	kind: PawnKind;
	id: string;
	name: string;
	image: string;
	visible: boolean;
	size: Size;
	width: number;
	height: number;
	hp: number;
	maxHp: number;
	ac: number;
}
export interface PlaceDeps {
	viewed: () => string;
	grid: () => Grid;
	send: (command: Outgoing) => void;
	announce: () => void;
}
export interface Place extends Tool {
	arm(armed: Armed | null): void;
	isArmed(): boolean;
}
export function createPlace(deps: PlaceDeps): Place {
	let armed: Armed | null = null;
	let pointer: Point | null = null;
	const snapped: Point = { x: 0, y: 0 };
	const ghost = blankDrawn();
	const shape: Ghostable = {
		id: "armed", kind: "object", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium", width: 0, height: 0, rotation: 0,
	};
	function shaped(next: Armed): Ghostable {
		const cell = Math.max(1, deps.grid().cellSize);
		const object = next.kind === "object";
		shape.kind = next.kind;
		shape.name = next.name;
		shape.image = next.image;
		shape.size = next.size;
		shape.width = object ? next.width || cell : 0;
		shape.height = object ? next.height || cell : 0;
		return shape;
	}
	function arm(next: Armed | null): void {
		armed = next;
		deps.announce();
	}
	function abandon(): boolean {
		if (!armed) {
			return false;
		}
		arm(null);
		return true;
	}
	function track(map: Point): void {
		pointer = { x: map.x, y: map.y };
	}
	return {
		press(map) {
			track(map);
			if (!armed) {
				return false;
			}
			const at = snapTo(deps.grid(), shaped(armed), map.x, map.y, snapped);
			const npc = armed.kind === "npc";
			const object = armed.kind === "object";
			deps.send({
				type: "pawn.spawn",
				kind: armed.kind,
				layer: deps.viewed(),
				x: at.x,
				y: at.y,
				visible: armed.visible,
				size: object ? undefined : armed.size,
				monsterId: armed.kind === "monster" ? armed.id : undefined,
				characterId: armed.kind === "player" ? armed.id : undefined,
				assetId: npc || object ? armed.id || undefined : undefined,
				name: npc || object ? armed.name : undefined,
				hp: npc ? armed.hp : undefined,
				maxHp: npc ? armed.maxHp : undefined,
				ac: npc ? armed.ac : undefined,
			});
			return true;
		},
		drag(map) {
			track(map);
		},
		release(map) {
			track(map);
		},
		cancel() {},
		secondary: abandon,
		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
		},
		key: () => false,
		abandon,
		active: () => armed !== null,
		contribute(out) {
			if (!armed || !pointer) {
				return;
			}
			const at = snapTo(deps.grid(), shaped(armed), pointer.x, pointer.y, snapped);
			out.ghosts.push(ghostOf(shape, at.x, at.y, ghost));
		},
		arm,
		isArmed: () => armed !== null,
	};
}
