// A range input with a live reading beside it.
//
// A SLIDER WITH NO NUMBER IS A GUESS. The thumb says roughly where you are
// between two ends, which is enough for a control you set by ear and not enough
// for one you want to set to the same place as last time -- and it is the only
// feedback a range gives, because a range has no text.
//
// ONE DELEGATED LISTENER RATHER THAN A HANDLER PER SLIDER, which is repeater.js'
// arrangement and is here for the same reason: these controls arrive in htmx
// swaps. The settings dialog is a fragment fetched into the content modal, so
// there is nothing on first load to bind to and nothing to re-bind when it is
// opened a second time.
//
// NOTHING IS PAINTED AT MOUNT, and that is not an omission. The server renders
// the reading with the value it rendered the slider at, so the two agree before
// any of this runs -- and a first paint here would be a second copy of that
// number in a second language, which is the drift this avoids by not having an
// opinion until somebody moves the thumb.
//
// THE SCRIPT WRITES A NUMBER AND THE MARKUP OWNS EVERYTHING ELSE. The per cent
// sign, the width, the alignment and the font are in the templ; what is in here
// is one text node. server/public/js is not a Tailwind source, so a class named
// in this file would never be emitted -- but the wording would not be emitted
// anywhere either, and a unit spelled in JavaScript is a unit nobody translating
// this app would find.
//
// A slider opts in by naming the element that reads it:
//
//     <input type="range" data-range-output="ping-volume-value">
//     <output id="ping-volume-value"><span data-range-value>40</span>%</output>
//
// A slider with no data-range-output is simply not one of these, which is a
// slider without a reading rather than a case to branch on.

document.addEventListener("input", (e) => {
    const input = e.target;
    if (!(input instanceof HTMLInputElement) || input.type !== "range") {
        return;
    }

    const id = input.dataset.rangeOutput;
    if (!id) {
        return;
    }

    // The slot rather than the output itself, because the output holds the unit
    // as well and replacing its text would eat the per cent sign.
    const slot = document.getElementById(id)?.querySelector("[data-range-value]");
    if (slot) {
        slot.textContent = input.value;
    }
});
