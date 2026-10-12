import { openDialog } from "./modal.js";
import { ALERT, PENDING_ALERT } from "./events.js";
const PENDING_KEY = PENDING_ALERT;
const dialog = document.getElementById("alert-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");
function show(heading, message) {
    headingEl.textContent = heading || "Something went wrong";
    messageEl.textContent = message || "";
    openDialog(dialog);
}
window.addEventListener(ALERT, (e) => {
    show(e.detail?.heading, e.detail?.message);
});
try {
    const pending = sessionStorage.getItem(PENDING_KEY);
    if (pending) {
        sessionStorage.removeItem(PENDING_KEY);
        const { heading, message } = JSON.parse(pending);
        show(heading, message);
    }
} catch {
}
