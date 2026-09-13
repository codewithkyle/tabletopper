import type { Grid, Tile, TileArt } from "../../protocol.ts";
import type { Stage, StageFactory } from "./stage.ts";
import type { Stamp } from "../../model/overlay.ts";
import { cellCentre } from "../../model/grid.ts";
import { createTerrainPass } from "../terrain-pass.ts";
import { watching } from "../../model/revisions.ts";
export const GHOST_ALPHA = 0.5;
export interface Laid {
	image: string;
	x: number;
	y: number;
	rotation: number;
}
export const terrainStage: StageFactory = (gl, resources): Stage => {
	const pass = createTerrainPass(gl, resources.pawnProgram);
	const ghosts = createTerrainPass(gl, resources.pawnProgram);
	const stamped = watching(["tiles", "table"]);
	const laid: Laid[] = [];
	const held: Laid[] = [];
	let layer = "";
	return {
		build(frame) {
			const grid = frame.state.table.grid;
			if (frame.rebuild || stamped.changed(frame.revisions) || frame.viewedID !== layer) {
				layer = frame.viewedID;
				placedTiles(frame.state.tiles, frame.state.table.palette, layer, grid, laid);
				pass.build(laid, grid, frame.resources.sprites);
			}
			heldStamps(frame.overlay.stamps, grid, held);
			ghosts.build(held, grid, frame.resources.sprites, GHOST_ALPHA);
		},
		draw(frame) {
			pass.draw(frame);
			ghosts.draw(frame);
		},
		visible: () => laid.length,
		dispose() {
			pass.dispose();
			ghosts.dispose();
		},
	};
};
export function placedTiles(
	tiles: readonly Tile[],
	palette: readonly TileArt[],
	layerID: string,
	grid: Grid,
	out: Laid[],
): Laid[] {
	let count = 0;
	for (const tile of tiles) {
		if (tile.layerId !== layerID) {
			continue;
		}
		const art = palette.find((entry) => entry.id === tile.art);
		if (!art) {
			continue;
		}
		const [x, y] = cellCentre(grid, tile.q, tile.r);
		count = write(out, count, art.image, x, y, tile.rotation);
	}
	out.length = count;
	return out;
}
export function heldStamps(stamps: readonly Stamp[], grid: Grid, out: Laid[]): Laid[] {
	let count = 0;
	for (const stamp of stamps) {
		if (stamp.image === "") {
			continue;
		}
		const [x, y] = cellCentre(grid, stamp.q, stamp.r);
		count = write(out, count, stamp.image, x, y, stamp.rotation);
	}
	out.length = count;
	return out;
}
function write(out: Laid[], at: number, image: string, x: number, y: number, rotation: number): number {
	const slot = out[at] ?? (out[at] = { image: "", x: 0, y: 0, rotation: 0 });
	slot.image = image;
	slot.x = x;
	slot.y = y;
	slot.rotation = rotation;
	return at + 1;
}
