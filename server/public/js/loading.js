// The loading bar. soft-loading.css shows it while <html state="loading">,
// and this module sets that attribute while any htmx request is in flight.
//
// Requests are counted, not flagged: two can overlap, and the bar has to
// stay up until the last one settles. Each request is remembered by its
// ctx -- htmx hands the same object to before: and finally: for one request
// -- so a finally: that was never preceded by a before: cannot drive the
// count negative, and requests settling out of order pair up correctly.
//
// This replaced env.js, a minified bundle from the old client that also
// sniffed the browser and the network connection into attributes nothing
// read, plus the ticket half of notif.js.

const inFlight = new WeakSet();
let count = 0;

function setState(state) {
    document.documentElement.setAttribute("state", state);
}

setState("idling");

// On `document` rather than `document.body`, and that distinction is
// load-bearing. htmx:finally:request is emitted after the swap; for a
// button that deleted its own card (hx-swap="delete") the button is gone by
// then, so htmx dispatches on document, and an event dispatched on document
// does not reach its own child body. document sees both the connected case,
// by bubbling, and the detached case, directly.
document.addEventListener("htmx:before:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx || inFlight.has(ctx)) {
        return;
    }
    inFlight.add(ctx);
    if (count++ === 0) {
        setState("loading");
    }
});

document.addEventListener("htmx:finally:request", (e) => {
    const ctx = e.detail?.ctx;
    if (!ctx || !inFlight.has(ctx)) {
        return;
    }
    inFlight.delete(ctx);
    if (--count === 0) {
        setState("idling");
    }
});
