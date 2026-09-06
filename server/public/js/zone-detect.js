// <zone-detect> preselects the browser's own time zone in the picker it wraps.
//
// IT SUGGESTS AND NEVER STORES. Reading the browser's zone and saving it behind
// the reader's back is exactly what this project turned down for the read path:
// the server renders dates and should not be guessing. Putting the same value
// in a <select> the reader is already looking at is a different thing -- they
// confirm it with the Save they were going to press, and if it is wrong the
// right answer is one dropdown away. So this element exists ONLY inside the
// welcome dialog, where nothing has been chosen yet, and never in the settings
// dialog, where something has.
//
// A custom element rather than a pass over the document, for the reason the
// deleted local-time.js gave: this markup arrives in an htmx swap, and an
// element upgrades itself on insertion with no rescan and no hook. Children are
// present both ways -- an element built by a swap is assembled before it is
// inserted, and on a first load customElements.define upgrades what the parser
// already finished.
//
// Doing nothing is a correct outcome and the common one on the edges: a zone
// Intl reports that this app does not offer leaves the server's default
// selected, which is the same answer the dialog would have shown anyway.
class ZoneDetect extends HTMLElement {
    connectedCallback() {
        const select = this.querySelector('select[name="timezone"]');
        if (!select) {
            return;
        }

        let zone;
        try {
            zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
        } catch {
            return;
        }
        if (!zone) {
            return;
        }

        // data-alias is the zone's older IANA spelling, and matching it is not
        // belt-and-braces. IANA renames zones and keeps the old name as a link,
        // and ICU -- which is what resolvedOptions() goes through -- still
        // canonicalises several to the pre-rename name: Asia/Kolkata comes back
        // as Asia/Calcutta, Europe/Kyiv as Europe/Kiev. Without this, detection
        // misses every reader in those places. The server renders the alias
        // beside the name; see internal/prefs.
        //
        // Walked rather than matched with an attribute selector, because a zone
        // name is data and building a selector out of it is a question about
        // escaping that this does not need to have.
        for (const option of select.options) {
            if (option.value === zone || option.dataset.alias === zone) {
                option.selected = true;
                return;
            }
        }
    }
}

customElements.define("zone-detect", ZoneDetect);
