import type { Event } from "../../protocol.ts";
import type { FrameContext } from "../frame-context.ts";
import type { Resources } from "../resources.ts";
export interface Stage {
	build?(frame: FrameContext): void;
	draw(frame: FrameContext): void;
	settling?(frame: FrameContext): boolean;
	event?(event: Event): void;
	stress?(count: number): number;
	showBlood?(on: boolean): void;
	fetched?(): number;
	dispose(): void;
}
export type StageFactory = (gl: WebGL2RenderingContext, resources: Resources) => Stage;
