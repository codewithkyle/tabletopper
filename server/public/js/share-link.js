const RESET_MS = 2000;
function copy(field) {
    if (!navigator.clipboard) {
        field.select();
        return Promise.reject(new Error("clipboard unavailable"));
    }
    return navigator.clipboard.writeText(field.value);
}
document.addEventListener("click", (event) => {
    const button = event.target.closest("[data-share-copy]");
    if (!button) {
        return;
    }
    const field = button.parentElement?.querySelector("[data-share-link]");
    if (!field) {
        return;
    }
    const idle = button.querySelector("[data-share-copy-idle]");
    const done = button.querySelector("[data-share-copy-done]");
    copy(field)
        .then(() => {
            idle.hidden = true;
            done.hidden = false;
            clearTimeout(button.dataset.resetTimer);
            button.dataset.resetTimer = setTimeout(() => {
                idle.hidden = false;
                done.hidden = true;
            }, RESET_MS);
        })
        .catch(() => {
        });
});
