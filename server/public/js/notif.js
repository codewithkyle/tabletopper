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

const loadingTokens = [];

document.body.addEventListener("htmx:before:request", () => {
    loadingTokens.push(env.startLoading());
});

document.body.addEventListener("htmx:after:request", (e) => {
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

// NOTE: unlike htmx:after:request this fires even when the request errored or
// was aborted, so the loading indicator cannot be left running.
document.body.addEventListener("htmx:finally:request", () => {
    while (loadingTokens.length > 0) {
        env.stopLoading(loadingTokens.pop());
    }
});
