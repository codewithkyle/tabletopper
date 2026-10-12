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
        for (const option of select.options) {
            if (option.value === zone || option.dataset.alias === zone) {
                option.selected = true;
                return;
            }
        }
    }
}
customElements.define("zone-detect", ZoneDetect);
