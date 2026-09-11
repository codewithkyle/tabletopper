// The rules every <dialog> in this app has to follow, in one place so no
// caller has to remember them.
//
// showModal() does not reset returnValue -- it survives from the previous
// open. A confirm accepted once and then dismissed with Escape the next time
// would still read "confirm" and fire the request again, so the reset has to
// happen on the way in, every time.
//
// showModal() on an already-open dialog throws InvalidStateError. Only the
// server-driven alert can hit that (an HX-Trigger arriving while its own
// dialog is up), but the guard costs a line and removes the whole class.
//
// The third line is not about dialogs at all. showModal() puts a dialog in the
// top layer, which paints over every z-index on the page, so any toast already
// on screen would spend the rest of its five seconds behind the scrim. The top
// layer is ordered by whatever was raised last, so raising the toasts again
// after the dialog puts them back on top; see toast.js for why they are up
// there in the first place. This is the only place in the app that opens a
// dialog, which is what makes one line here enough.
import { raiseToasts } from "./toast.js";
export function openDialog(dialog) {
    dialog.returnValue = "";
    if (!dialog.open) {
        dialog.showModal();
    }
    raiseToasts();
}
