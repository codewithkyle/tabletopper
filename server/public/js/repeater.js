// The delete buttons on the character sheet's repeaters -- info rows and
// spell cards. A button carrying data-remove-closest names the ancestor to
// remove:
//
//     <button type="button" data-remove-closest="[data-info-row]">
//
// One delegated listener rather than a handler per button, so rows htmx
// appends later are covered without anything re-binding. This is all that
// was left of Alpine: two x-on:click attributes, which was not enough to
// carry the library and its MutationObserver on every page.
//
// REMOVING THE ROW IS NOT THE WHOLE OPERATION ANY MORE. It was, while the
// editor had a Save button: the form was the only model, and the next save
// posted whatever rows were left. The editor now autosaves per panel and has
// no Save button, so a removal nothing tells the server about would come back
// on the next page load.
//
// So the panel is told. The form is read before the row goes -- the button is
// inside the row being removed, so `closest` stops working a line later -- and
// `repeater:changed` is named in each repeater panel's hx-trigger. It carries
// no delay, unlike the debounced `input` beside it: a deletion is a finished
// action, not a keystroke in a run of them.
//
// The create form is untouched by this. It triggers on submit and listens for
// no such event, so the dispatch there lands on nothing.
document.addEventListener("click", (e) => {
    const button = e.target.closest("[data-remove-closest]");
    if (!button) {
        return;
    }

    const form = button.closest("form");
    button.closest(button.dataset.removeClosest)?.remove();
    form?.dispatchEvent(new CustomEvent("repeater:changed", { bubbles: true }));
});
