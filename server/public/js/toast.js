import { TOAST } from "./events.js";
const PENDING_KEY = TOAST;
const DEFAULT_SECONDS = 5;
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
function raise(host) {
    if (!canRaise) {
        return;
    }
    if (host.matches(":popover-open")) {
        host.hidePopover();
    }
    host.showPopover();
}
export function raiseToasts() {
    if (shell && shell.isConnected) {
        raise(shell);
    }
}
export function toast(message, seconds = DEFAULT_SECONDS) {
    const el = document.createElement("output");
    el.role = "status";
    el.textContent = message;
    const host = getShell();
    raise(host);
    const before = host.offsetHeight;
    host.appendChild(el);
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
try {
    const pending = sessionStorage.getItem(PENDING_KEY);
    if (pending) {
        sessionStorage.removeItem(PENDING_KEY);
        toast(pending);
    }
} catch {
}
document.addEventListener("htmx:after:request", (e) => {
    const headers = e.detail?.ctx?.response?.headers;
    const trigger = headers?.get("HX-Trigger");
    if (!trigger) {
        return;
    }
    let events;
    try {
        events = JSON.parse(trigger);
    } catch {
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
