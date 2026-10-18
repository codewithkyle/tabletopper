import { openDialog } from "./modal.js";
import { MODAL_CLOSE, MODAL_OPEN } from "./events.js";
const dialog = document.getElementById("content-modal");
const box = dialog.querySelector("[data-modal-box]");
const body = dialog.querySelector("[data-modal-body]");
const loadingEl = dialog.querySelector("[data-modal-loading]");
const errorEl = dialog.querySelector("[data-modal-error]");
const retryEl = dialog.querySelector("[data-modal-retry]");
const SIZES = { sm: "24rem", md: "32rem", lg: "42rem", xl: "56rem" };
const FRAGMENT_PREFIX = "/fragment/";
let current = null;
let lastUrl = null;
function show(el) {
    loadingEl.hidden = el !== loadingEl;
    errorEl.hidden = el !== errorEl;
    body.hidden = el !== body;
}
function focusContent() {
    body
        .querySelector("input:not([type=hidden]), select, textarea, button, a[href]")
        ?.focus();
}
function load(url) {
    lastUrl = url;
    current = null;
    show(loadingEl);
    body.replaceChildren();
    htmx.ajax("GET", url, {
        target: body,
        source: dialog,
    });
}
document.addEventListener("click", (e) => {
    const trigger = e.target.closest("[data-modal-open]");
    if (!trigger) {
        return;
    }
    window.dispatchEvent(
        new CustomEvent(MODAL_OPEN, {
            detail: {
                url: trigger.dataset.modalOpen,
                size: trigger.dataset.modalSize,
            },
        }),
    );
});
window.addEventListener(MODAL_OPEN, (e) => {
    const url = e.detail?.url;
    if (!url) {
        console.error("modal:open fired without a url", e.detail);
        return;
    }
    if (!url.startsWith(FRAGMENT_PREFIX)) {
        console.error(
            `modal:open needs a ${FRAGMENT_PREFIX} url, refusing:`,
            url,
        );
        return;
    }
    box.style.maxWidth = SIZES[e.detail.size] ?? SIZES.md;
    openDialog(dialog);
    load(url);
});
window.addEventListener(MODAL_CLOSE, () => {
    dialog.close();
});
retryEl.addEventListener("click", () => {
    if (lastUrl) {
        load(lastUrl);
    }
});
dialog.addEventListener("htmx:before:request", (e) => {
    if (e.target !== dialog) {
        return;
    }
    current = e.detail.ctx;
});
dialog.addEventListener("htmx:before:swap", (e) => {
    if (e.target === dialog && e.detail.ctx !== current) {
        e.preventDefault();
    }
});
dialog.addEventListener("htmx:finally:request", (e) => {
    if (e.target !== dialog || e.detail.ctx !== current) {
        return;
    }
    const status = e.detail.ctx.response?.status;
    const loaded = status !== undefined && status < 400;
    show(loaded ? body : errorEl);
    if (loaded) {
        focusContent();
    }
});
dialog.addEventListener("close", () => {
    current = null;
    const fading = dialog.getAnimations().map((a) => a.finished);
    Promise.allSettled(fading).then(() => {
        if (!dialog.open) {
            body.replaceChildren();
        }
    });
});
const autoOpen = document.querySelector("[data-modal-autoopen]");
if (autoOpen) {
    window.dispatchEvent(
        new CustomEvent(MODAL_OPEN, {
            detail: {
                url: autoOpen.dataset.modalAutoopen,
                size: autoOpen.dataset.modalSize,
            },
        }),
    );
}
