document.addEventListener("click", (e) => {
    const button = e.target.closest("[data-remove-closest]");
    if (!button) {
        return;
    }
    const form = button.closest("form");
    button.closest(button.dataset.removeClosest)?.remove();
    form?.dispatchEvent(new CustomEvent("repeater:changed", { bubbles: true }));
});
