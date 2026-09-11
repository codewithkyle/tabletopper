// The room page's menu bar. Loaded by that page and no other.
//
// THE TOOL PILL USED TO BE HERE AND IS NOT ANY MORE. It kept its own
// aria-pressed and nothing read it; now that two of its modes are real, the
// state belongs in the bundle that has the canvas in it --
// server/js/room/tools.ts -- because a control and the thing it controls in two
// scripts that cannot import each other is a contract with nobody to enforce it.
//
// THE BAR IS SEVEN <details> ELEMENTS AND THIS MAKES THEM BEHAVE LIKE A MENU
// BAR. A <details> opens and closes on its own with no script at all, which is
// why it is the element: with this file blocked the menus still work, they just
// work one at a time and stay open until they are clicked again. What is added
// here is the four behaviours a desktop menu bar has and HTML does not -- only
// one menu open at once, a click anywhere else closes it, Escape closes it, and
// once one is open the others open on hover as the pointer travels along.
//
// NO CLASS NAME IS WRITTEN IN THIS FILE. server/public/js is deliberately not a
// Tailwind @source, so a class written here would never be emitted -- see the
// note at the bottom of css/app.css. What it toggles is a property the browser
// owns, details.open, and the styling for that is rendered in templ.
import { toast } from "./toast.js";
import { ROOM_BLOOD, ROOM_VIEW } from "./events.js";
const bar = document.querySelector("[data-room-bar]");
// The bar is absent on every page but the room's, and this module is only
// loaded there -- but a page that loads it and renders no bar should do nothing
// rather than throw on the first query.
if (bar) {
    wireMenus(bar);
}
function wireMenus(root) {
    const menus = () => root.querySelectorAll("[data-room-menu]");
    function close(except) {
        for (const el of menus()) {
            if (el !== except) {
                el.open = false;
            }
        }
    }
    // `toggle` does not bubble, so this listens in the capture phase. Opening
    // one menu closes the rest, which is the whole of "only one at a time".
    root.addEventListener(
        "toggle",
        (e) => {
            if (e.target.matches("[data-room-menu]") && e.target.open) {
                close(e.target);
            }
        },
        true,
    );
    // Once a menu is open, travelling along the bar opens the next one without
    // a second click. This is the behaviour that makes a menu bar feel like a
    // menu bar rather than seven unrelated dropdowns -- and it deliberately
    // does nothing while they are all shut, so a pointer crossing the bar on
    // its way somewhere else opens nothing.
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
    // pointerdown rather than click, so the menu is gone before whatever was
    // clicked underneath it responds.
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
        // A summary is the menu's own handle; the browser is opening or
        // closing it and this must not fight that.
        if (e.target.closest("summary")) {
            return;
        }
        const item = e.target.closest("[data-room-action]");
        if (item) {
            run(item.dataset.roomAction, item.dataset.roomValue);
        }
        // Choosing anything closes the menu, whether it was one of the actions
        // below, a link, or an htmx post. The lock item answers by swapping
        // itself, which lands in a menu that is already shut and is correct the
        // next time it opens.
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
// THE VIEW ITEMS AND THE CAMERA ARE IN DIFFERENT BUNDLES. This file is served
// as it is written; the renderer is TypeScript bundled into
// /static/room.js from server/js/room/. Neither can import the other, so what
// crosses between them is a window event -- the same shape as the PENDING_ALERT
// contract between the room bundle and alert-modal.js.
//
// THE EVENT NAME IS THE CONTRACT and both sides import it from events.js. The
// listener is in server/js/room/render/renderer.ts; the values are the ones
// pages.viewMenu sends, and an unknown one is ignored there rather than here,
// because the renderer is what knows which of them it can honour.
//
// A room with no canvas -- a closed one, or a browser without WebGL2 -- has no
// listener, and the event lands nowhere. That is the right amount of nothing to
// happen: the menu item is still there, still says what it does, and the reason
// it did not is on the table in front of them.
const VIEW_EVENT = ROOM_VIEW;
function view(action) {
    if (!action) {
        return;
    }
    window.dispatchEvent(new CustomEvent(VIEW_EVENT, { detail: { action } }));
}
// Clearing the blood crosses the same gap and carries nothing, because there is
// only one thing it can mean.
//
// IT IS A VIEWER'S OWN AND IT IS NOT A COMMAND TO THE ROOM. Every mark on the
// floor was drawn by this browser out of hit points it watched change, so there
// is nothing on the server to delete and nobody else's table to touch -- which
// is why this is a window event and not the hx-post every other destructive
// item in the bar is. Somebody who wants it back plays on; the next hit bleeds.
//
// THERE IS NO CONFIRMATION IN FRONT OF IT for the same reason. The confirm
// modal is for what cannot be undone, and this undoes nothing that was ever
// anywhere else.
const BLOOD_EVENT = ROOM_BLOOD;
function clearBlood() {
    window.dispatchEvent(new CustomEvent(BLOOD_EVENT));
}
// navigator.clipboard is only defined in a secure context, which is https and
// localhost. There is no field to select as a fallback the way the share dialog
// has, so the honest answer is to put the code where it can be read and copied
// by hand rather than to claim it was copied.
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
// The whole document rather than the table region, so the menu bar and the tool
// pill come with it -- a fullscreen table you cannot reach the menus from is a
// fullscreen table you have to leave to do anything.
//
// Both calls reject rather than throw when the browser refuses, which it does
// when the request did not come from a gesture. Nothing is reported: the user
// pressed a menu item and the screen did not change, which is its own answer.
function toggleFullscreen() {
    if (document.fullscreenElement) {
        document.exitFullscreen().catch(() => {});
        return;
    }
    document.documentElement.requestFullscreen().catch(() => {});
}
