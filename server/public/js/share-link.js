// Click-to-copy for the share dialog's link field.
//
// A DELEGATED LISTENER ON THE DOCUMENT, because the button does not exist when
// this module runs: it arrives inside a fragment htmx swaps into the content
// modal, and again on every create and revoke. Binding at load would catch the
// first one and none of the rest; rebinding on a swap would need a hook the
// modal does not offer.
//
// THE COPIED STATE IS TWO PRE-RENDERED SPANS AND THE hidden ATTRIBUTE, not a
// class and not new text. server/public/js is deliberately not a Tailwind
// source, so a class name written here would never be emitted -- the same
// reason the journal toolbar reports itself through aria-pressed.
const RESET_MS = 2000;

// navigator.clipboard is only defined in a secure context, which is https and
// localhost. Selecting the field is the honest fallback everywhere else: the
// reader still gets the link, they just press the keys themselves, and nothing
// claims to have copied something it did not.
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

    // Scoped to the button's own field rather than the document's, so a second
    // link on a page later would copy itself rather than the first one.
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
            // The timer is cleared and restarted on the button rather than
            // held in module scope, so two rapid clicks do not leave the first
            // timeout to reset a label the second one just set.
            clearTimeout(button.dataset.resetTimer);
            button.dataset.resetTimer = setTimeout(() => {
                idle.hidden = false;
                done.hidden = true;
            }, RESET_MS);
        })
        .catch(() => {
            // Nothing to report: the field is selected and the label still
            // says Copy, which is the truth.
        });
});
