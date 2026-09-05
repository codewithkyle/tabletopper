// The delete buttons on the character sheet's repeaters -- info rows and
// spell cards. A button carrying data-remove-closest names the ancestor to
// remove:
//
//     <button type="button" data-remove-closest="[data-info-row]">
//
// No request is made and nothing has to be told: the form is the only model
// (see info-rows-table.templ), so removing the row is the whole operation.
//
// One delegated listener rather than a handler per button, so rows htmx
// appends later are covered without anything re-binding. This is all that
// was left of Alpine: two x-on:click attributes, which was not enough to
// carry the library and its MutationObserver on every page.
document.addEventListener("click", (e) => {
    const button = e.target.closest("[data-remove-closest]");
    if (!button) {
        return;
    }
    button.closest(button.dataset.removeClosest)?.remove();
});
