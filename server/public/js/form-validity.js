// Suppresses the browser's own validation bubble, and only the bubble.
//
// htmx validates a form before it posts one -- `request.validate` defaults to
// true when the element carrying hx-post IS a form -- and it does it by calling
// form.reportValidity(). That is the behaviour we want: an invalid panel must
// not reach the server. What comes with it is not. reportValidity() pops a
// native tooltip and pulls focus to the first invalid control, which is fine
// once, on a deliberate submit, and unbearable on a debounced autosave: clear a
// required field, pause for a second, and the page yanks the caret back out of
// whatever you moved on to.
//
// preventDefault on `invalid` drops the tooltip and the focus pull. It does not
// change what reportValidity() returns, so htmx still abandons the request and
// the panel still does not save.
//
// Nothing is lost by this, because the field is already saying so. Every field
// in form-field.templ carries DaisyUI's .validator, which colours the control
// off :user-invalid and reveals a .validator-hint underneath it -- styling the
// app renders itself, in the app's own type, with no JavaScript and no request.
// The native bubble was the second, worse copy of a message already on screen.
//
// Capture phase: `invalid` does not bubble, so a listener on document only sees
// it on the way down.
document.addEventListener(
    "invalid",
    (e) => {
        e.preventDefault();
    },
    true,
);
