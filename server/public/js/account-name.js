import { SETTINGS_CHANGE } from "./events.js";
const greeting = document.getElementById("account-name");
if (greeting) {
    window.addEventListener(SETTINGS_CHANGE, (e) => {
        const name = e.detail?.name ?? "";
        if (name !== "") {
            greeting.textContent = name;
        }
    });
}
