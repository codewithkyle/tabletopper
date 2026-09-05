import alerts from "./alerts.js";
import env from "./env.js";

env.boot();

// A toast raised on a response that also redirects would be destroyed by the
// navigation, so it is parked here and picked up on the next load.
const PENDING_TOAST_KEY = "flash:toast";

window.addEventListener("toast", (e) => {
    alerts.toast(e.detail?.msg ?? e.detail.value);
});

const pendingToast = sessionStorage.getItem(PENDING_TOAST_KEY);
if (pendingToast) {
    alerts.toast(pendingToast);
    sessionStorage.removeItem(PENDING_TOAST_KEY);
}

// Every htmx listener here is on `document`, not `document.body`, and that
// distinction is load-bearing.
//
// htmx dispatches its events on the requesting element, but falls back when that
// element has gone away:
//
//     let n = e?.isConnected ? e : document; n.dispatchEvent(s)
//
// htmx:finally:request is emitted from a finally block that runs *after* the
// swap. Both delete buttons in this app are `hx-target="closest *-card"
// hx-swap="delete"`, so by then the button has been removed along with its card,
// isConnected is false, and the event goes to `document`. Listeners on
// document.body never saw it -- bubbling travels upward, so an event dispatched
// on document does not reach its own child. The ticket was never released and
// the loading bar ran until the next navigation. htmx:after:request was
// unaffected, because it is emitted before the swap while the button is still
// connected, which is what made this look like it only involved the confirm
// modal.
//
// document sits above both paths: it receives the connected case by bubbling and
// the detached case by direct dispatch.
//
// Tickets are keyed by the request ctx rather than kept in a stack. htmx hands
// the same ctx object to before: and finally: for one request, so this pairs
// correctly when several requests are in flight and settle out of order -- which
// a stack could not do, and which is also why env.stopLoading has to splice one
// element rather than truncate.
const loadingTokens = new WeakMap();

document.addEventListener("htmx:before:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx) {
        return;
    }
    loadingTokens.set(ctx, env.startLoading());
});

document.addEventListener("htmx:finally:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx) {
        return;
    }
    const token = loadingTokens.get(ctx);
    if (!token) {
        return;
    }
    loadingTokens.delete(ctx);
    env.stopLoading(token);
});

document.addEventListener("htmx:after:request", (e) => {
    // NOTE: htmx 4 runs on fetch(), so there is no xhr on the event detail --
    // the response and its headers hang off the request ctx instead.
    const headers = e.detail?.ctx?.response?.headers;
    if (!headers) {
        return;
    }

    const trigger = headers.get("HX-Trigger");
    if (!trigger) {
        return;
    }

    let events = {};
    try {
        events = JSON.parse(trigger);
    } catch {
        // NOTE: HX-Trigger is allowed to be a bare event name rather than JSON,
        // and htmx dispatches that form itself
        return;
    }

    const toast = events?.[PENDING_TOAST_KEY];
    if (!toast) {
        return;
    }

    if (headers.get("HX-Redirect") || headers.get("HX-Refresh")) {
        sessionStorage.setItem(PENDING_TOAST_KEY, toast);
    } else {
        alerts.toast(toast);
    }
});
