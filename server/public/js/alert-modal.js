// The <dialog id="alert-modal"> in the base layout, driven entirely by the
// server: internal/helpers/htmx.go writes an "alert" HX-Trigger on the error
// responses, and htmx dispatches it here.
//
// Deliberately a separate dialog from the confirm one rather than a second
// mode of it. An alert can arrive while a confirm is waiting to be answered --
// they are independent responses -- and sharing an element would mean the
// alert overwrites the confirm's text and steals its close event, settling
// htmx's promise with whichever button the user pressed on the wrong message.
import { openDialog } from "./modal.js";

const dialog = document.getElementById("alert-modal");
const headingEl = dialog.querySelector("[data-modal-heading]");
const messageEl = dialog.querySelector("[data-modal-message]");

// On `window`: htmx dispatches HX-Trigger events on the requesting element
// with bubbles set, so they climb to window; when that element has already
// been swapped away htmx dispatches on document instead, which is also on the
// path to window. Both cases land here.
window.addEventListener("alert", (e) => {
    headingEl.textContent = e.detail?.heading ?? "Something went wrong";
    messageEl.textContent = e.detail?.message ?? "";
    openDialog(dialog);
});
