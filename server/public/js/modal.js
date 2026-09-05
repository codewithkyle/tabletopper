// The two rules every <dialog> in this app has to follow, in one place so no
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
export function openDialog(dialog) {
    dialog.returnValue = "";
    if (!dialog.open) {
        dialog.showModal();
    }
}
