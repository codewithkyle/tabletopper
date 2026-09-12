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
import { ownPawnsStage } from "./own-pawns.ts";
import { ownAurasStage } from "./own-auras.ts";
import { ownRingsStage } from "./own-rings.ts";
import { pawnsStage } from "./pawns.ts";
import { pingsStage } from "./pings.ts";
import { ringsStage } from "./rings.ts";
import { strokesStage } from "./strokes.ts";
import { tilesStage } from "./tiles.ts";
const NAMES: ReadonlyMap<StageFactory, string> = new Map([
	[tilesStage, "tiles"],
	[gridStage, "grid"],
	[decalsStage, "decals"],
	[strokesStage, "strokes"],
	[fogStage, "fog"],
	[floorMarksStage, "floor-marks"],
	[aurasStage, "auras"],
	[pawnsStage, "pawns"],
	[ringsStage, "rings"],
	[ghostsStage, "ghosts"],
	[handlesStage, "handles"],
	[pingsStage, "pings"],
	[ownAurasStage, "own-auras"],
	[ownPawnsStage, "own-pawns"],
	[ownRingsStage, "own-rings"],
	[overMarksStage, "over-marks"],
]);
export function nameOf(factory: StageFactory): string {
	return NAMES.get(factory) ?? "stage";
}
export function stagesFor(role: Role): readonly StageFactory[] {
	return [
		tilesStage, gridStage, decalsStage, strokesStage,
		...(role === "gm" ? [fogStage] : []),
		floorMarksStage, aurasStage, pawnsStage, ringsStage, ghostsStage,
		handlesStage, pingsStage,
		...(role === "player" ? [fogStage, ownAurasStage, ownPawnsStage, ownRingsStage] : []),
		overMarksStage,
	];
}
