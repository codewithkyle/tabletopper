document.addEventListener("input", (e) => {
    const input = e.target;
    if (!(input instanceof HTMLInputElement) || input.type !== "range") {
        return;
    }
    const id = input.dataset.rangeOutput;
    if (!id) {
        return;
    }
    const slot = document.getElementById(id)?.querySelector("[data-range-value]");
    if (slot) {
        slot.textContent = input.value;
    }
});
