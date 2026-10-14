import { openDialog } from "./modal.js";
const dialog = document.getElementById("confirm-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");
const confirmEl = dialog.querySelector("[data-modal-confirm]");
const cancelEl = dialog.querySelector("[data-modal-cancel]");
const DEFAULT_MESSAGE =
    "This action may affect your data without the ability to be undone.";
let pending = null;
function ask(elt, message, settle) {
    if (pending) {
        settle.abandon();
        return;
    }
    pending = settle;
    headingEl.textContent = elt.dataset?.confirmHeading ?? "Are you sure?";
    messageEl.textContent = message || DEFAULT_MESSAGE;
    confirmEl.textContent = elt.dataset?.confirmLabel ?? "Confirm";
    cancelEl.textContent = elt.dataset?.cancelLabel ?? "Close";
    openDialog(dialog);
}
export function confirmThen(elt, action) {
    ask(elt, elt.getAttribute("hx-confirm"), { proceed: action, abandon() {} });
}
document.addEventListener("htmx:confirm", (e) => {
    const elt = e.detail?.ctx?.sourceElement;
    if (!elt || !elt.hasAttribute("hx-confirm")) {
        return;
    }
    e.preventDefault();
    ask(elt, e.detail?.ctx?.confirm, {
        proceed: () => e.detail.issueRequest(),
        abandon: () => e.detail.dropRequest(),
    });
});
dialog.addEventListener("close", () => {
    const settle = pending;
    pending = null;
    if (!settle) {
        return;
    }
    if (dialog.returnValue === "confirm") {
        settle.proceed();
    } else {
        settle.abandon();
    }
});
