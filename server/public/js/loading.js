const inFlight = new WeakSet();
let count = 0;
function isBackground(target) {
    return target instanceof Element && target.hasAttribute("data-quiet");
}
function setState(state) {
    document.documentElement.setAttribute("state", state);
}
setState("idling");
document.addEventListener("htmx:before:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx || inFlight.has(ctx) || isBackground(e.target)) {
        return;
    }
    inFlight.add(ctx);
    if (count++ === 0) {
        setState("loading");
    }
});
document.addEventListener("htmx:finally:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx || !inFlight.has(ctx)) {
        return;
    }
    inFlight.delete(ctx);
    if (--count === 0) {
        setState("idling");
    }
});
