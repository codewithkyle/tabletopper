// What happens to somebody the GM removes from the room.
//
// THE SERVER HAS ALREADY DONE THE REAL WORK by the time this runs. The room
// emitted player.kicked to this person alone, the hub is closing their sockets
// with the reason "kicked" so the client stops reconnecting, and their session
// rows no longer name the room. What is left is telling them, which the server
// cannot do: the page they are looking at is not being re-rendered, and the
// socket is about to go.
//
// SO THE MESSAGE IS PARKED AND THE BROWSER NAVIGATES. This is the shape
// toast.js already uses for a message that has to outlive a navigation -- write
// it to sessionStorage, let the next page pick it up -- and the reason is the
// same: a dialog opened a moment before the page changes is a dialog nobody
// reads.
//
// THE KEY IS A CONTRACT WITH server/public/js/alert-modal.js, which is the
// other half of this and is in a different bundle; both import it from
// public/js/events.js, and a test holds the two bundles to that.
import { PENDING_ALERT } from "../../public/js/events.js";

// The heading is ours and the message is the server's. room.KickReason is a
// sentence chosen for the person reading it, and it travels on the event so
// that the wording lives with the rule rather than being written out again
// here -- if the GM is ever given a reason to type, this shows it with no
// change.
const KICK_HEADING = "Removed from the room";

// leaveKicked parks the alert and goes home.
//
// THE NAVIGATION IS NOT CONDITIONAL ON THE PARKING. A private window can refuse
// sessionStorage, and being left sitting on a dead room page is a worse outcome
// than arriving at the homepage with no explanation -- the socket is closed,
// the session no longer names the room, and every control on the page is about
// to fail.
export function leaveKicked(reason: string): void {
	try {
		sessionStorage.setItem(
			PENDING_ALERT,
			JSON.stringify({ heading: KICK_HEADING, message: reason }),
		);
	} catch {
		// A private window, or storage the browser has turned off. The
		// navigation below still has to happen.
	}

	location.assign("/");
}
