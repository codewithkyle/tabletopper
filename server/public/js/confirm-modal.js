// Replaces htmx's window.confirm() with the <dialog id="confirm-modal"> in the
// base layout. htmx hands us a promise to settle: exactly one of issueRequest
// or dropRequest must be called, or the element's request queue stalls.
import { openDialog } from "./modal.js";

const dialog = document.getElementById("confirm-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");
const confirmEl = dialog.querySelector("[data-modal-confirm]");
const cancelEl = dialog.querySelector("[data-modal-cancel]");

const DEFAULT_MESSAGE =
    "This action may affect your data without the ability to be undone.";

// There is one dialog, so there is at most one unsettled confirm.
let pending = null;

// On `document` for the same reason loading.js is -- htmx dispatches to document
// whenever the requesting element is detached, and those never reach body. The
// element is still connected at confirm time, so this one is consistency
// rather than a fix, but the rule is easier to keep than the exception.
document.addEventListener("htmx:confirm", (e) => {
    // NOTE: htmx 4 moved the triggering element and the hx-confirm message onto
    // the request ctx; the old detail.elt and detail.question are gone.
    const elt = e.detail?.ctx?.sourceElement;
    if (!elt || !elt.hasAttribute("hx-confirm")) {
        return;
    }

    e.preventDefault();

    // showModal() makes the rest of the page inert, so a user cannot open a
    // second confirm over the first. A programmatic trigger still can. Drop
    // the newcomer rather than overwrite `pending`: the overwritten one would
    // never be settled and its element would stop making requests entirely.
    if (pending) {
        e.detail.dropRequest();
        return;
    }
    pending = e.detail;

    headingEl.textContent = elt.dataset?.confirmHeading ?? "Are you sure?";
    messageEl.textContent = e.detail?.ctx?.confirm ?? DEFAULT_MESSAGE;
    confirmEl.textContent = elt.dataset?.confirmLabel ?? "Confirm";
    cancelEl.textContent = elt.dataset?.cancelLabel ?? "Close";

    openDialog(dialog);
});

// Every way out of the dialog ends here: both buttons submit the
// method="dialog" form, and Escape closes it directly. Settling in one place
// is what makes it impossible to leave htmx's promise hanging -- the previous
// version settled in two click handlers and had no Escape path at all, so it
// never had to answer this.
//
// Anything that is not the confirm button is a cancel, including Escape, which
// leaves returnValue as the empty string openDialog() reset it to.
dialog.addEventListener("close", () => {
    const settle = pending;
    pending = null;
    if (!settle) {
        return;
    }
    if (dialog.returnValue === "confirm") {
        settle.issueRequest();
    } else {
        settle.dropRequest();
    }
});
