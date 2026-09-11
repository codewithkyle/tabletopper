

















import { PENDING_ALERT } from "../../public/js/events.js";






const KICK_HEADING = "Removed from the room";








export function leaveKicked(reason: string): void {
	try {
		sessionStorage.setItem(
			PENDING_ALERT,
			JSON.stringify({ heading: KICK_HEADING, message: reason }),
		);
	} catch {
		
		
	}

	location.assign("/");
}
