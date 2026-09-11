import type { FrameContext } from "../frame-context.ts";
import type { Pawn } from "../../protocol.ts";
import { aboveFog } from "../../model/polygon.ts";
export function lifted(frame: FrameContext, pawn: Pick<Pawn, "x" | "y" | "ownerId">): boolean {
	return aboveFog(pawn, frame.state.fog, frame.viewed, frame.role, frame.user);
}
