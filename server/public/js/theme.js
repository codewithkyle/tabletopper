// The palette, repainted on the page the reader is already looking at.
//
// THE THEME IS SERVER-RENDERED AND THIS IS NOT WHAT APPLIES IT. templ/layouts
// writes data-theme onto <html> for every page, which is what makes the choice
// survive a reload with no flash of the wrong colours. This handles exactly one
// moment: the settings dialog saved, the response swapped a fragment inside the
// dialog, and the <html> element it did not touch is still carrying the old
// answer. Without this the reader picks Dark, closes the dialog, and sits on a
// light page until they navigate.
//
// internal/htmx raises it as {"theme:change": {"palette": "..."}}, on window for
// the reason alert-modal.js gives: htmx dispatches HX-Trigger events on the
// requesting element with bubbles set, and on document when that element has
// been swapped away, so window is on both paths.
//
// An empty palette is the "system" setting and means removing the attribute
// rather than writing one, because the OS preference lives in a CSS media query
// that only applies while :root carries no data-theme.
window.addEventListener("theme:change", (e) => {
    const palette = e.detail?.palette ?? "";

    if (palette) {
        document.documentElement.dataset.theme = palette;
    } else {
        delete document.documentElement.dataset.theme;
    }
});
