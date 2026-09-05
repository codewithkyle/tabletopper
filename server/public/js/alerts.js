// Thin facade over the toaster so callers don't reach into it directly.
//
// This used to also front snackbar.js and notifications.js with
// alert/success/warn/error methods. Those were only ever called from the
// service-worker branch of bootstrap.js, which was removed along with the
// service worker it depended on -- so the whole notification stack went with
// it. Server-side alerts go to the Alpine <modal-element> in base.templ via
// the "alert" HX-Trigger; see internal/helpers/htmx.go.
import toaster from "./toaster.js";

class Alerts {
    toast(message, duration = 5) {
        toaster.push({ message: message, duration: duration });
    }
}

const alerts = new Alerts();
export default alerts;
