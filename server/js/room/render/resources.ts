import type { GlyphAtlas } from "./glyphs.ts";
import type { PawnProgram } from "./pawn-pass.ts";
import type { RingProgram } from "./ring-pass.ts";
import type { SpriteCache } from "./sprites.ts";
import { createGlyphAtlas } from "./glyphs.ts";
import { createPawnProgram } from "./pawn-pass.ts";
import { createRingProgram } from "./ring-pass.ts";
import { createSpriteCache } from "./sprites.ts";
export interface Resources {
	readonly sprites: SpriteCache;
	invalidate(): void;
	readonly atlas: GlyphAtlas | null;
	readonly pawnProgram: PawnProgram;
	readonly ringProgram: RingProgram;
	begin(rebuild: boolean): void;
	end(): boolean;
	dispose(): void;
}
export function createResources(gl: WebGL2RenderingContext, invalidate: () => void): Resources {
	const sprites = createSpriteCache(gl, invalidate);
	const atlas = createGlyphAtlas(gl);
	const pawnProgram = createPawnProgram(gl);
	const ringProgram = createRingProgram(gl);
	return {
		sprites,
		invalidate,
		atlas,
		pawnProgram,
		ringProgram,
		begin(rebuild) {
			sprites.begin(rebuild);
		},
		end() {
			return sprites.end();
		},
		dispose() {
			sprites.dispose();
			atlas?.dispose();
			pawnProgram.dispose();
			ringProgram.dispose();
		},
	};
}
