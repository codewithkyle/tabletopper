// Replaces htmx's window.confirm() with the Alpine-driven <modal-element> in the
// base layout. htmx hands us a promise to settle: exactly one of issueRequest or
// dropRequest must be called, or the element's request queue stalls.
const pendingConfirms = {};

document.body.addEventListener("htmx:confirm", (e) => {
    // NOTE: htmx 4 moved the triggering element and the hx-confirm message onto
    // the request ctx; the old detail.elt and detail.question are gone.
    const elt = e.detail?.ctx?.sourceElement;
    if (!elt || !elt.hasAttribute("hx-confirm")) {
        return;
    }

    e.preventDefault();

    const uid = crypto.randomUUID();
    pendingConfirms[uid] = {
        issueRequest: e.detail.issueRequest,
        dropRequest: e.detail.dropRequest,
    };

    window.dispatchEvent(
        new CustomEvent("show-confirm-modal", {
            detail: {
                confirmLabel: elt.dataset?.confirmLabel ?? "Confirm",
                cancelLabel: elt.dataset?.cancelLabel ?? "Close",
                heading: elt.dataset?.confirmHeading ?? "Are you sure?",
                message:
                    e.detail?.ctx?.confirm ??
                    "This action may affect your data without the ability to be undone.",
                uid,
            },
        }),
    );
});

function settle(uid, action) {
    const pending = uid ? pendingConfirms[uid] : null;
    if (!pending) {
        console.error("Failed to find confirm modal callback", uid);
        return;
    }
    delete pendingConfirms[uid];
    pending[action]();
}

window.addEventListener("confirm-modal:confirm", (e) => {
    settle(e?.detail ?? null, "issueRequest");
});

window.addEventListener("confirm-modal:cancel", (e) => {
    settle(e?.detail ?? null, "dropRequest");
});
