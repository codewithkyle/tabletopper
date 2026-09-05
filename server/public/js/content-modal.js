// The reusable <dialog id="content-modal"> in the base layout. Fire a
// "modal:open" event carrying a URL and it fetches that URL through htmx and
// swaps the response into the dialog:
//
//     $dispatch('modal:open', { url: '/characters/new', size: 'lg' })
//
// Alpine's $dispatch sets bubbles, so it reaches the window listener below
// from anywhere on the page; plain dispatchEvent on window works the same.
// Accepted detail keys: url (required), method (default GET), values, size.
//
// The content is server-rendered and its actions are ordinary htmx -- because
// htmx performs the swap itself, hx-* attributes inside the response are
// processed on arrival with no htmx.process() call here. A response that
// should dismiss the dialog says so with an HX-Trigger of {"modal:close":true}.
import { openDialog } from "./modal.js";

const dialog = document.getElementById("content-modal");
const box = dialog.querySelector("[data-modal-box]");
const body = dialog.querySelector("[data-modal-body]");
const loadingEl = dialog.querySelector("[data-modal-loading]");
const errorEl = dialog.querySelector("[data-modal-error]");
const retryEl = dialog.querySelector("[data-modal-retry]");

// Widths are inline styles rather than max-w-* utilities because
// server/public/js is deliberately not a Tailwind @source -- see the note at
// the bottom of css/app.css. A class name written in here would never be
// emitted, and the modal would silently keep DaisyUI's default width. "md" is
// that default (.modal-box is max-width: 32rem), so an event with no size
// behaves exactly as it would without this.
const SIZES = { sm: "24rem", md: "32rem", lg: "42rem", xl: "56rem" };

// The ctx of the fetch whose content the dialog is currently expecting.
//
// htmx's default hx-sync is "queue first" per source element, and every load
// here names the same source, so a load started while another is in flight is
// queued rather than cancelled -- closing and reopening during a slow fetch
// would otherwise land the first response in the second modal. Comparing the
// ctx is the same trick notif.js uses to pair its loading tickets: htmx hands
// the identical object to every event of one request.
let current = null;
let lastRequest = null;

// Exactly one of the three is visible. The attribute rather than a class
// because preflight's [hidden] carries !important, so it wins over the display
// utilities these elements already have.
function show(el) {
    loadingEl.hidden = el !== loadingEl;
    errorEl.hidden = el !== errorEl;
    body.hidden = el !== body;
}

function load(request) {
    lastRequest = request;
    current = null;
    show(loadingEl);
    body.replaceChildren();

    htmx.ajax(request.method ?? "GET", request.url, {
        target: body,
        // Naming the dialog as the source puts every event for this fetch on
        // the dialog itself. That is how the listeners below tell the content
        // load apart from requests the content makes once it has landed --
        // those bubble through the dialog too, but with their own target.
        source: dialog,
        values: request.values,
    });
}

window.addEventListener("modal:open", (e) => {
    if (!e.detail?.url) {
        console.error("modal:open fired without a url", e.detail);
        return;
    }
    box.style.maxWidth = SIZES[e.detail.size] ?? SIZES.md;
    openDialog(dialog);
    load(e.detail);
});

// The dismissal signal for server-driven flows: a form in the modal posts,
// succeeds, and the response closes the dialog with an HX-Trigger header
// rather than the client guessing from the status code. A validation failure
// just re-renders the form and says nothing, so the modal stays open.
window.addEventListener("modal:close", () => {
    dialog.close();
});

retryEl.addEventListener("click", () => {
    if (lastRequest) {
        load(lastRequest);
    }
});

dialog.addEventListener("htmx:before:request", (e) => {
    if (e.target !== dialog) {
        return;
    }
    current = e.detail.ctx;
});

dialog.addEventListener("htmx:before:swap", (e) => {
    // A queued or in-flight load from an earlier open. Let it run to
    // completion so htmx settles its own bookkeeping, but do not let it paint
    // over what is on screen now.
    if (e.target === dialog && e.detail.ctx !== current) {
        e.preventDefault();
    }
});

dialog.addEventListener("htmx:finally:request", (e) => {
    if (e.target !== dialog || e.detail.ctx !== current) {
        return;
    }
    // finally: runs after the swap, so by here the body holds the response.
    //
    // A missing response means the fetch threw -- offline, DNS, a timeout. A
    // 4xx or 5xx arrives here having swapped nothing, because the noSwap
    // config in base.templ covers both ranges; without this branch the spinner
    // would run until the next navigation.
    const status = e.detail.ctx.response?.status;
    show(status !== undefined && status < 400 ? body : errorEl);
});

// Reset on the way out, but leave the content alone: .modal-box fades over
// 0.3s after close() and clearing it here would empty the box mid-fade.
// load() clears it on the way back in, which is the only moment it matters.
dialog.addEventListener("close", () => {
    // Any load still in flight belongs to the modal that just closed.
    current = null;
    box.style.maxWidth = "";
});
