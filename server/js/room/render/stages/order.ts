import type { Role } from "../../protocol.ts";
import type { StageFactory } from "./stage.ts";
import { aurasStage } from "./auras.ts";
import { decalsStage } from "./decals.ts";
import { floorMarksStage } from "./floor-marks.ts";
import { fogStage } from "./fog.ts";
import { ghostsStage } from "./ghosts.ts";
import { gridStage } from "./grid.ts";
import { handlesStage } from "./handles.ts";
import { overMarksStage } from "./over-marks.ts";
import { pawnsStage } from "./pawns.ts";
import { pingsStage } from "./pings.ts";
import { ringsStage } from "./rings.ts";
import { strokesStage } from "./strokes.ts";
import { tilesStage } from "./tiles.ts";
export function stagesFor(role: Role): readonly StageFactory[] {
	return [
		tilesStage, gridStage, decalsStage, strokesStage,
		...(role === "gm" ? [fogStage] : []),
		floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
		handlesStage, pingsStage,
		...(role === "player" ? [fogStage] : []),
		overMarksStage,
	];
}
