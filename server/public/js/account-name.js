// The greeting on the homepage, corrected when the settings dialog renames the
// account underneath it.
//
// IT IS LOADED BY ONE PAGE BECAUSE ONE PAGE HAS THE ELEMENT. That is the whole
// argument for it being a file of its own rather than another listener in the
// shell: the room, the roster, the manual and the asset pages never greet
// anybody by name, and a script in the base layout would be a request every one
// of them made for an element none of them has.
//
// THIS USED TO BE AN OUT-OF-BAND SWAP AND COULD NOT STAY ONE. The save's reply
// rendered <span id="account-name"> with hx-swap-oob, which htmx reports as an
// error when nothing on the page matches -- fine while the homepage was the only
// place the dialog could be opened from, and wrong the moment the room's Help
// menu could open it too. An event is what a page is allowed not to care about.
//
// internal/htmx raises it as {"settings:change": {"name": "...", ...}}, on
// window for the reason theme.js gives: htmx dispatches HX-Trigger events on the
// requesting element with bubbles set, and on document when that element has
// been swapped away, so window is on both paths. The other fields in the detail
// are the tabletop's and are read in server/js/room.
//
// AN EMPTY NAME IS NOT A RENAME. The save refuses a blank display name before it
// writes anything, so nothing should ever arrive here empty -- and if something
// did, leaving the greeting alone is better than greeting nobody.
const greeting = document.getElementById("account-name");

if (greeting) {
    window.addEventListener("settings:change", (e) => {
        const name = e.detail?.name ?? "";

        if (name !== "") {
            greeting.textContent = name;
        }
    });
}
