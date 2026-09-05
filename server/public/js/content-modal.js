// The reusable <dialog id="content-modal"> in the base layout. Fire a
// "modal:open" event carrying a URL and it fetches that URL through htmx and
// swaps the response into the dialog:
//
//     window.dispatchEvent(new CustomEvent("modal:open", {
//         detail: { url: "/fragment/character/new", size: "lg" },
//     }));
//
// Dispatching on an element works too if the event bubbles. Accepted detail
// keys: url (required) and size -- that is the whole surface.
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

// Every load has to name a /fragment/ route, and the modal checks rather than
// trusting the caller. This is the only place in the app that takes a URL as
// data -- the detail of an event any component can fire -- so without the check
// a dispatch naming a page URL swaps a whole <html> document into a <div>
// inside .modal-box, and the wreckage reads as a styling bug rather than a
// wrong URL. The prefix is what makes "is this a page or a piece of one?"
// answerable here at all; see the fragment block in routes.go.
//
// Comparing against a leading slash rules out an absolute or protocol-relative
// URL for free: "//elsewhere.example/fragment/x" does not start with it.
const FRAGMENT_PREFIX = "/fragment/";

// The ctx of the fetch whose content the dialog is currently expecting.
//
// htmx's default hx-sync is "queue first" per source element, and every load
// here names the same source, so a load started while another is in flight is
// queued rather than cancelled -- closing and reopening during a slow fetch
// would otherwise land the first response in the second modal. Comparing the
// ctx is the same trick loading.js uses to pair its loading tickets: htmx hands
// the identical object to every event of one request.
let current = null;
let lastUrl = null;

// Exactly one of the three is visible. The attribute rather than a class
// because preflight's [hidden] carries !important, so it wins over the display
// utilities these elements already have.
function show(el) {
    loadingEl.hidden = el !== loadingEl;
    errorEl.hidden = el !== errorEl;
    body.hidden = el !== body;
}

// showModal() hands focus to the first focusable thing in the dialog, and while
// the fetch is in flight that is the Close button of the loading state -- which
// this then hides, dropping focus to the dialog itself. So focus follows the
// content in: the first control of the fetched fragment takes it, which for a
// dialog asking one question is the field it is asking in.
function focusContent() {
    body
        .querySelector("input:not([type=hidden]), select, textarea, button, a[href]")
        ?.focus();
}

// GET is not a default here, it is the only option. A /fragment/ route is a
// GET that returns partial HTML and nothing else, so a method or a body on the
// opening fetch has nowhere to land: the subtree catch-all in routes.go takes any
// other verb and answers 404. Posting belongs to the actions inside the fetched
// markup, and those are ordinary hx-* attributes carried by the fragment.
function load(url) {
    lastUrl = url;
    current = null;
    show(loadingEl);
    body.replaceChildren();

    htmx.ajax("GET", url, {
        target: body,
        // Naming the dialog as the source puts every event for this fetch on
        // the dialog itself. That is how the listeners below tell the content
        // load apart from requests the content makes once it has landed --
        // those bubble through the dialog too, but with their own target.
        source: dialog,
    });
}

window.addEventListener("modal:open", (e) => {
    const url = e.detail?.url;
    if (!url) {
        console.error("modal:open fired without a url", e.detail);
        return;
    }
    // Refused before openDialog, so a bad dispatch leaves the dialog shut
    // rather than opening it onto a spinner that never resolves.
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

// The dismissal signal for server-driven flows: a form in the modal posts,
// succeeds, and the response closes the dialog with an HX-Trigger header
// rather than the client guessing from the status code. A validation failure
// just re-renders the form and says nothing, so the modal stays open.
window.addEventListener("modal:close", () => {
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
    const loaded = status !== undefined && status < 400;
    show(loaded ? body : errorEl);
    if (loaded) {
        focusContent();
    }
});

// NOTHING VISIBLE IS RESET HERE. .modal-box fades for 0.3s after close(), so
// anything this handler changes is changed while the box is still on screen and
// the user watches it happen: clearing the content would empty the box mid-fade,
// and clearing the width made a 24rem dialog jump to the 32rem default and widen
// as it faded out.
//
// Neither needs undoing anyway, because both are set on the way in -- load()
// replaces the content, and the modal:open handler always assigns a width rather
// than only assigning one when a size was asked for.
dialog.addEventListener("close", () => {
    // Any load still in flight belongs to the modal that just closed.
    current = null;
});
