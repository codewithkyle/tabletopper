const callback = {};
document.body.addEventListener("htmx:confirm", (e) => {
    const target = e.detail.elt;
    if (!target || !target.hasAttribute("hx-confirm")) return;
    e.preventDefault();
    const uid = crypto.randomUUID();
    callback[uid] = {
        event: e,
        issueRequest: e.detail.issueRequest,
    }
    window.dispatchEvent(new CustomEvent("show-confirm-modal", {
        detail: {
            confirmLabel: target.dataset?.confirmLabel ?? "Confirm",
            cancelLabel: target.dataset?.cancelLabel ?? "Close",
            heading: target.dataset?.confirmHeading ?? "Are you sure?",
            message: e.detail?.question ?? "This action may affect your data without the ability to be undone.",
            uid: uid,
        }
    }));
});
window.addEventListener("confirm-modal:confirm", (e) => {
    const uid = e?.detail ?? null;
    if (!uid || !(uid in callback)) {
        console.error("Failed to find confirm modal callback", uid);
        return;
    }
    console.log("do delete");
    callback[uid]?.issueRequest(true);
    delete callback[uid];
});
window.addEventListener("confirm-modal:cancel", (e) => {
    const uid = e.detail;
    if (!uid || !(uid in callback)) {
        console.error("Failed to find confirm modal callback", uid);
        return;
    }
    delete callback[uid];
});
