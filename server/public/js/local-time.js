// <local-time> puts a server-rendered timestamp into the reader's own locale
// and time zone:
//
//     <local-time><time datetime="2026-09-05T18:04:11Z">5 Sep 2026, 18:04 UTC</time></local-time>
//
// The server writes both halves because it holds an instant and nothing else.
// MySQL runs at +00:00 and the connection parses times, so a handler has a UTC
// time.Time and no idea where the reader is -- only the browser knows that. The
// datetime attribute is the machine-readable value this reads; the text inside
// is the server's UTC rendering, which is what shows before this module runs,
// with JavaScript off, and in a test.
//
// A custom element rather than a pass over the document at load, so a card
// swapped in by htmx upgrades itself with no rescan and no hook, and because
// soft-loading and toaster-component are already this shape.
//
// Intl is the whole implementation. An undefined locale is the browser's own,
// so a reader in Sydney gets Sydney time in Australian order and no locale file
// is ever shipped. dayjs would have needed its localizedFormat plugin plus a
// file per language to do the same, and its timezone plugin converts by calling
// this API anyway.
const formats = {
    short: new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
    }),
    // The full styles carry the zone name, so hovering answers "whose 6pm?".
    full: new Intl.DateTimeFormat(undefined, {
        dateStyle: "full",
        timeStyle: "long",
    }),
};

class LocalTime extends HTMLElement {
    // Reads the attribute and never the text, so this is idempotent: an element
    // moved in the DOM connects a second time and formats the same instant
    // again, rather than trying to parse the prose it wrote last time.
    connectedCallback() {
        const el = this.querySelector("time[datetime]");
        if (!el) {
            return;
        }

        const at = new Date(el.getAttribute("datetime"));
        // An unparseable value leaves the server's own rendering on screen,
        // which is a correct date in the wrong zone rather than "Invalid Date".
        if (Number.isNaN(at.getTime())) {
            return;
        }

        el.textContent = formats.short.format(at);
        el.title = formats.full.format(at);
    }
}

customElements.define("local-time", LocalTime);
