import { PENDING_ALERT } from "../../public/js/events.js";
const KICK_HEADING = "Removed from the room";
const CLOSED_HEADING = "Room closed";
const CLOSED_MESSAGE = "The host closed this room.";
export function leaveKicked(reason: string): void {
	leave(KICK_HEADING, reason);
}
export function leaveClosed(): void {
	leave(CLOSED_HEADING, CLOSED_MESSAGE);
}
function leave(heading: string, message: string): void {
	try {
		sessionStorage.setItem(PENDING_ALERT, JSON.stringify({ heading, message }));
	} catch {
	}
	location.assign("/");
}
