import { THEME_CHANGE } from "./events.js";
window.addEventListener(THEME_CHANGE, (e) => {
    const palette = e.detail?.palette ?? "";
    if (palette) {
        document.documentElement.dataset.theme = palette;
    } else {
        delete document.documentElement.dataset.theme;
    }
});
