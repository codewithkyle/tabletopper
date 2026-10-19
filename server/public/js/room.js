import { toast } from "./toast.js";
import { ROOM_BLOOD, ROOM_CHARACTER, ROOM_VIEW } from "./events.js";
import { sheetChannel } from "./sheet-channel.js";
const bar = document.querySelector("[data-room-bar]");
if (bar) {
    wireMenus(bar);
}
const sheets = sheetChannel();
window.addEventListener(ROOM_CHARACTER, (e) => {
    sheets?.postMessage({ id: e.detail.id });
});
function wireMenus(root) {
    const menus = () => root.querySelectorAll("[data-room-menu]");
    function close(except) {
        for (const el of menus()) {
            if (el !== except) {
                el.open = false;
            }
        }
    }
    root.addEventListener(
        "toggle",
        (e) => {
            if (e.target.matches("[data-room-menu]") && e.target.open) {
                close(e.target);
            }
        },
        true,
    );
    root.addEventListener("pointerover", (e) => {
        const summary = e.target.closest("summary");
        if (!summary) {
            return;
        }
        const details = summary.parentElement;
        if (details.open || !root.querySelector("[data-room-menu][open]")) {
            return;
        }
        details.open = true;
    });
    document.addEventListener("pointerdown", (e) => {
        if (!root.contains(e.target)) {
            close();
        }
    });
    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape") {
            close();
        }
    });
    root.addEventListener("click", (e) => {
        if (e.target.closest("summary")) {
            return;
        }
        const item = e.target.closest("[data-room-action]");
        if (item) {
            run(item.dataset.roomAction, item.dataset.roomValue);
        }
        if (e.target.closest("li")) {
            close();
        }
    });
}
function run(action, value) {
    switch (action) {
        case "copy-code":
            copyCode(value);
            break;
        case "view":
            view(value);
            break;
        case "clear-blood":
            clearBlood();
            break;
        case "fullscreen":
            toggleFullscreen();
            break;
        default:
            console.error("unknown room action:", action);
    }
}
const VIEW_EVENT = ROOM_VIEW;
function view(action) {
    if (!action) {
        return;
    }
    window.dispatchEvent(new CustomEvent(VIEW_EVENT, { detail: { action } }));
}
const BLOOD_EVENT = ROOM_BLOOD;
function clearBlood() {
    window.dispatchEvent(new CustomEvent(BLOOD_EVENT));
}
function copyCode(code) {
    if (!code) {
        return;
    }
    if (!navigator.clipboard) {
        toast(`Room code: ${code}`);
        return;
    }
    navigator.clipboard.writeText(code).then(
        () => toast("Room code copied."),
        () => toast(`Room code: ${code}`),
    );
}
function toggleFullscreen() {
    if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
        return;
    }
    document.documentElement.requestFullscreen().catch(() => {});
}
