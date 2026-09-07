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
//
// The shell is a popover, and that is the whole answer to "why does a toast
// paint underneath the dialogs". A <dialog> opened with showModal() is
// promoted to the browser's top layer, which paints above every stacking
// context on the page no matter what z-index any of them asked for: the
// toaster asks for 2000 and still loses to a dialog that asks for nothing.
// There is no number that wins, because it is not a contest about numbers.
// The only way above a top-layer element is to be one, and popover="manual"
// is the way in that is not modal -- it does not trap focus, dim the page,
// take a backdrop, or block a single thing underneath it.
//
// WITHIN THE TOP LAYER THE ORDER IS PROMOTION ORDER, last one raised on top,
// and that is why raise() exists rather than a one-off showPopover(). A shell
// promoted when the first toast of the session was raised sits below every
// dialog opened after it, so it is re-raised as each toast arrives, and
// openDialog() in modal.js re-raises it after showModal() to cover the other
// order -- a toast already on screen when a dialog opens over it.
//
// WHAT THIS DOES NOT BUY BACK IS THE CLICK. A modal dialog makes the rest of
// the document inert, the top layer included, so while one is open the toast
// is visible but untouchable: click-to-dismiss below does nothing and the
// timer is what clears it. Both halves of that were measured in Chromium
// rather than reasoned about -- the toast paints over the dialog, and a hit
// test at the same point lands on the dialog behind it.

const PENDING_KEY = "flash:toast";
const DEFAULT_SECONDS = 5;

// Whether this browser has the top layer on offer. Everything above is a
// no-op without it, and a toast that renders under a dialog is what the app
// did before: worse than the fix, better than a TypeError that loses the
// message altogether.
const canRaise = "popover" in HTMLElement.prototype;

let shell = null;

function getShell() {
    if (!shell || !shell.isConnected) {
        shell = document.createElement("toaster-component");
        if (canRaise) {
            shell.popover = "manual";
        }
        document.body.appendChild(shell);
    }
    return shell;
}

// hidePopover() throws on a popover that is not showing, so the state has to
// be asked for. Hiding and showing again in the same task moves the shell to
// the end of the top layer without the browser ever computing the closed
// style in between, which is why the toasts already standing in it do not
// flicker or restart their animations.
function raise(host) {
    if (!canRaise) {
        return;
    }
    if (host.matches(":popover-open")) {
        host.hidePopover();
    }
    host.showPopover();
}

// Put the toasts back on top of a dialog that has just opened over them.
// Called by openDialog(), which is the one place in the app that opens one.
// It never builds a shell: with nothing on screen there is nothing to raise.
export function raiseToasts() {
    if (shell && shell.isConnected) {
        raise(shell);
    }
}

export function toast(message, seconds = DEFAULT_SECONDS) {
    const el = document.createElement("output");
    el.role = "status";
    // textContent, not innerHTML: the message usually contains a name the
    // user typed, and a name is text.
    el.textContent = message;

    const host = getShell();
    raise(host);
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
