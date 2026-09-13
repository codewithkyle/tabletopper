const TOGGLE = "[data-notify-permission]";
const HINT = "[data-notify-blocked]";
function reveal(box) {
    const note = box.closest("div")?.querySelector(HINT);
    if (!(note instanceof HTMLElement)) {
        return;
    }
    const granted = "Notification" in window && Notification.permission === "granted";
    note.hidden = !box.checked || granted;
}
function ask(box) {
    if (!box.checked || !("Notification" in window) || Notification.permission !== "default") {
        reveal(box);
        return;
    }
    Promise.resolve(Notification.requestPermission()).then(
        () => reveal(box),
        () => reveal(box),
    );
}
document.addEventListener("change", (e) => {
    const box = e.target;
    if (box instanceof HTMLInputElement && box.matches(TOGGLE)) {
        ask(box);
    }
});
document.addEventListener("htmx:after:swap", (e) => {
    const root = e.target;
    if (!(root instanceof Element)) {
        return;
    }
    for (const box of root.querySelectorAll(TOGGLE)) {
        reveal(box);
    }
});
