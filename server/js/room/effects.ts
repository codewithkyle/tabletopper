import type { Event, Role } from "./protocol.ts";
export type Effect = (event: Event) => void;
export function fanOut(effects: readonly Effect[]): Effect {
	return (event) => {
		for (const effect of effects) {
			effect(event);
		}
	};
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
export interface DepartureDeps {
	role: Role;
	kicked(reason: string): void;
	closed(): void;
}
export function departures(deps: DepartureDeps): Effect {
	return (event) => {
		if (event.type === "player.kicked") {
			deps.kicked(event.reason);
			return;
		}
		if (event.type === "room.closed" && deps.role !== "gm") {
			deps.closed();
		}
	};
}
