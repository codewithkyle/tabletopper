import { PENDING_ALERT } from "../../public/js/events.js";
const KICK_HEADING = "Removed from the room";
const CLOSED_HEADING = "Room closed";
const CLOSED_MESSAGE = "The host closed this room.";
const EXPIRED_HEADING = "Left the room";
const EXPIRED_MESSAGE = "You signed out or joined another room in this browser, so this tab left the room.";
export function leaveKicked(reason: string): void {
	leave(KICK_HEADING, reason);
}
export function leaveClosed(): void {
	leave(CLOSED_HEADING, CLOSED_MESSAGE);
}
export function leaveExpired(): void {
	leave(EXPIRED_HEADING, EXPIRED_MESSAGE, "/rooms");
}
function leave(heading: string, message: string, destination = "/"): void {
	try {
		sessionStorage.setItem(PENDING_ALERT, JSON.stringify({ heading, message }));
	} catch {
	}
	location.assign(destination);
}
