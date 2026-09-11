










import type { Event } from "./protocol.ts";

export type Effect = (event: Event) => void;


export function fanOut(effects: readonly Effect[]): Effect {
	return (event) => {
		for (const effect of effects) {
			effect(event);
		}
	};
}



























export function touchesPawns(type: Event["type"]): boolean {
	if (type === "snapshot" || type === "table.updated" || type.startsWith("fog.")) {
		return true;
	}

	return type.startsWith("pawn.") && type !== "pawn.dragging";
}

export interface RefusalDeps {
	
	alert(heading: string, message: string): void;

	
	
	resync(): void;
}











export function refusals(deps: RefusalDeps): Effect {
	return (event) => {
		if (event.type !== "error") {
			return;
		}

		if (event.code === "not_found") {
			deps.resync();

			return;
		}

		deps.alert(event.heading, event.message);
	};
}
