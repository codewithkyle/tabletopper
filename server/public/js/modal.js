import { raiseToasts } from "./toast.js";
export function openDialog(dialog) {
    dialog.returnValue = "";
    if (!dialog.open) {
        dialog.showModal();
    }
    raiseToasts();
}
