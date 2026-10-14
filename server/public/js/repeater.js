import { confirmThen } from "./confirm-modal.js";
document.addEventListener("click", (e) => {
    const button = e.target.closest("[data-remove-closest]");
    if (!button) {
        return;
    }
    if (button.hasAttribute("hx-confirm")) {
        confirmThen(button, () => remove(button));
        return;
    }
    remove(button);
});
function remove(button) {
    if (!button.isConnected) {
        return;
    }
    const form = button.closest("form");
    button.closest(button.dataset.removeClosest)?.remove();
    form?.dispatchEvent(new CustomEvent("repeater:changed", { bubbles: true }));
}
