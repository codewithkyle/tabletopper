// Toasts. A message becomes one <output role="status"> inside the
// <toaster-component> shell, which toast.css positions and animates; the
// shell is created on first use so a page with no toasts carries no markup.
//
// The server raises a toast with an HX-Trigger header carrying
// {"flash:toast": "..."} (see internal/htmx). htmx dispatches that as a
// DOM event too, but the header is read here directly, because the same
// response may also carry HX-Redirect or HX-Refresh -- and a toast shown a
// moment before the page navigates away is never seen. In that case it is
// parked in sessionStorage and shown on the next load instead. The event
// alone cannot tell us which case we are in; the headers can.
//
// This replaced toaster.js, alerts.js and the toast half of notif.js: a
// minified bundle from the old client, a one-method facade over it, and a
// requestAnimationFrame countdown loop, for what is a timer per element.

const PENDING_KEY = "flash:toast";
const DEFAULT_SECONDS = 5;

let shell = null;

function getShell() {
    if (!shell || !shell.isConnected) {
        shell = document.createElement("toaster-component");
        document.body.appendChild(shell);
    }
    return shell;
}

export function toast(message, seconds = DEFAULT_SECONDS) {
    const el = document.createElement("output");
    el.role = "status";
    // textContent, not innerHTML: the message usually contains a name the
    // user typed, and a name is text.
    el.textContent = message;

    const host = getShell();
    const before = host.offsetHeight;
    host.appendChild(el);
    // Slide the new toast up out of the space it just added, so the stack
    // grows smoothly rather than jumping.
    const grown = host.offsetHeight - before;
    el.animate(
        [{ transform: `translateY(${grown}px)` }, { transform: "translateY(0)" }],
        { duration: 150, easing: "ease-out" },
    );

    const timer = setTimeout(() => el.remove(), seconds * 1000);
    el.addEventListener("click", () => {
        clearTimeout(timer);
        el.remove();
    });
}

// A toast parked by the previous page.
try {
    const pending = sessionStorage.getItem(PENDING_KEY);
    if (pending) {
        sessionStorage.removeItem(PENDING_KEY);
        toast(pending);
    }
} catch {
    // sessionStorage can throw in a private window; a lost toast is fine.
}

// On `document`, not `document.body`: htmx dispatches on the requesting
// element, but falls back to document when that element has already been
// swapped away, and an event dispatched on document never reaches body.
document.addEventListener("htmx:after:request", (e) => {
    // htmx 4 runs on fetch(), so the response hangs off the request ctx.
    const headers = e.detail?.ctx?.response?.headers;
    const trigger = headers?.get("HX-Trigger");
    if (!trigger) {
        return;
    }

    let events;
    try {
        events = JSON.parse(trigger);
    } catch {
        // HX-Trigger may be a bare event name rather than JSON; htmx
        // dispatches that form itself and it is never a toast.
        return;
    }

    const message = events?.[PENDING_KEY];
    if (!message) {
        return;
    }

    if (headers.get("HX-Redirect") || headers.get("HX-Refresh")) {
        try {
            sessionStorage.setItem(PENDING_KEY, message);
        } catch {
            toast(message);
        }
    } else {
        toast(message);
    }
});
