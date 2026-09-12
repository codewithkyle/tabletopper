import type { Drawn } from "../../model/overlay.ts";
import type { PawnPulse } from "../pawn-pass.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import { createPawnPass } from "../pawn-pass.ts";
import { concealed } from "../../model/polygon.ts";
import { lifted } from "./lifted.ts";
import { fastBeat, slowBeat } from "../../model/health.ts";
import { stressPawns } from "../stress.ts";
import { healthOf } from "../../model/health.ts";
export const pawnsStage: StageFactory = (gl, resources): Stage => {
	const pass = createPawnPass(gl, resources.pawnProgram);
	const pulse: PawnPulse = { slow: 0, heart: 0 };
	const drawn: Drawn[] = [];
	let synthetic: Drawn[] = [];
	let frame: FrameContext | null = null;
	let cell = 1;
	let centreX = 0;
	let centreY = 0;
	const hidden = (pawn: Pawn): boolean => {
		if (!frame) {
			return false;
		}
		return concealed(pawn, frame.state.fog, frame.viewed, frame.role, frame.user)
			|| lifted(frame, pawn);
	};
	return {
		build(now) {
			frame = now;
			cell = now.cell;
			const map = now.viewed?.map ?? null;
			centreX = map ? map.width / 2 : 0;
			centreY = map ? map.height / 2 : 0;
			if (!now.rebuild) {
				return;
			}
			visiblePawns(now.state.pawns, now.viewedID, drawn, hidden);
			pass.build(
				synthetic.length > 0 ? drawn.concat(synthetic) : drawn,
				now.state.table.grid,
				resources.sprites,
			);
		},
		draw(now) {
			pulse.slow = slowBeat(now.now);
			pulse.heart = fastBeat(now.now);
			pass.draw(now, pulse);
		},
		settling: () => pass.beating(),
		stress(count) {
			synthetic = count > 0 ? stressPawns(count, drawn, cell, centreX, centreY) : [];
			return synthetic.length;
		},
		dispose: () => pass.dispose(),
	};
};

export function visiblePawns(
	pawns: readonly Pawn[], layerID: string, out: Drawn[],
	concealed?: (pawn: Pawn) => boolean,
): Drawn[] {
	let count = 0;
	for (const pawn of pawns) {
		if (pawn.layerId !== layerID) {
			continue;
		}
		if (concealed?.(pawn)) {
			continue;
		}
		const drawn = out[count] ?? (out[count] = blank());
		drawn.id = pawn.id;
		drawn.kind = pawn.kind;
		drawn.name = pawn.name;
		drawn.image = pawn.image;
		drawn.x = pawn.x;
		drawn.y = pawn.y;
		drawn.z = pawn.z;
		drawn.size = pawn.size;
		drawn.width = pawn.width;
		drawn.height = pawn.height;
		drawn.rotation = pawn.rotation;
		drawn.hidden = !pawn.visible;
		drawn.health = healthOf(pawn);
		count++;
	}
	out.length = count;
	return out;
}
function blank(): Drawn {
	return {
		id: "",
		kind: "monster",
		name: "",
		image: "",
		x: 0,
		y: 0,
		z: 0,
		size: "medium",
		width: 0,
		height: 0,
		rotation: 0,
		hidden: false,
		health: null,
	};
}
