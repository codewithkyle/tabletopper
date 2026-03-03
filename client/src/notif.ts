import notifications from "~brixi/controllers/alerts";
import env from "~brixi/controllers/env";

env.boot();

window.addEventListener("toast", (event: CustomEvent) => {
    notifications.toast(event.detail?.msg ?? event.detail.value);
})
;
const initialToast = sessionStorage.getItem("flast:toast");
if (initialToast) {
    notifications.toast(initialToast);
    sessionStorage.removeItem("flast:toast");
}

const tickets = [];
document.body.addEventListener("htmx:beforeRequest", (e) => {
    const ticket = env.startLoading();
    tickets.push(ticket);
});
document.body.addEventListener("htmx:afterRequest", (e:CustomEvent) => {
    const { xhr } = e.detail;
    const triggerHeader = xhr.getResponseHeader("HX-Trigger");
    const refreshHeader = xhr.getResponseHeader("HX-Refresh");
    const redirectHeader = xhr.getResponseHeader("HX-Redirect");
    let triggerHeaderParsed = {};
    if (triggerHeader) {
        triggerHeaderParsed = JSON.parse(triggerHeader);
    }
    if (triggerHeaderParsed?.["flash:toast"]) {
        if (redirectHeader || refreshHeader) {
            sessionStorage.setItem("flast:toast", triggerHeaderParsed["flash:toast"]);
        } else {
            notifications.toast(triggerHeaderParsed["flash:toast"]);
        }
    }
    for (let i = tickets.length - 1; i >= 0; i--) {
        env.stopLoading(tickets[i]);
    }
});
