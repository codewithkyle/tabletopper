import { openDialog } from "./modal.js";
const dialog = document.getElementById("confirm-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");
const confirmEl = dialog.querySelector("[data-modal-confirm]");
const cancelEl = dialog.querySelector("[data-modal-cancel]");
const DEFAULT_MESSAGE =
    "This action may affect your data without the ability to be undone.";
let pending = null;
document.addEventListener("htmx:confirm", (e) => {
    const elt = e.detail?.ctx?.sourceElement;
    if (!elt || !elt.hasAttribute("hx-confirm")) {
        return;
    }
    e.preventDefault();
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
