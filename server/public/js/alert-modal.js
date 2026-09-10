// The <dialog id="alert-modal"> in the base layout, driven entirely by the
// server: internal/htmx writes an ALERT HX-Trigger on the error
// responses, and htmx dispatches it here.
//
// Deliberately a separate dialog from the confirm one rather than a second
// mode of it. An alert can arrive while a confirm is waiting to be answered --
// they are independent responses -- and sharing an element would mean the
// alert overwrites the confirm's text and steals its close event, settling
// htmx's promise with whichever button the user pressed on the wrong message.
import { openDialog } from "./modal.js";
import { ALERT, PENDING_ALERT } from "./events.js";

// PENDING_KEY is an alert parked by the page BEFORE this one, for the case the
// server has no response to hang an HX-Trigger on: the room client is told over
// its socket that the GM has removed this person, and then sends them here.
// A dialog opened a moment before a navigation is a dialog nobody reads, so the
// message crosses the navigation in sessionStorage instead -- the same shape
// toast.js uses for a toast that arrives with an HX-Redirect.
//
// THE KEY IS A CONTRACT WITH server/js/room/exit.ts, which writes it from the
// other bundle; both import it from events.js.
const PENDING_KEY = PENDING_ALERT;

const dialog = document.getElementById("alert-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");

function show(heading, message) {
    headingEl.textContent = heading || "Something went wrong";
    messageEl.textContent = message || "";
    openDialog(dialog);
}

// On `window`: htmx dispatches HX-Trigger events on the requesting element
// with bubbles set, so they climb to window; when that element has already
// been swapped away htmx dispatches on document instead, which is also on the
// path to window. Both cases land here.
window.addEventListener(ALERT, (e) => {
    show(e.detail?.heading, e.detail?.message);
});

// IT IS READ ONCE AND REMOVED FIRST, so a parked alert cannot survive into a
// third page if anything below it throws -- an alert that reappeared on every
// navigation would be worse than one that was lost.
try {
    const pending = sessionStorage.getItem(PENDING_KEY);
    if (pending) {
        sessionStorage.removeItem(PENDING_KEY);
        const { heading, message } = JSON.parse(pending);
        show(heading, message);
    }
} catch {
    // sessionStorage throws in a private window and JSON.parse throws on
    // anything that is not ours. A lost alert is better than a page that
    // stopped loading over one.
}
