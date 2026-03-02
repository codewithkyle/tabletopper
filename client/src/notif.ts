import notifications from "~brixi/controllers/alerts";
import env from "~brixi/controllers/env";

env.boot();

window.addEventListener("toast", (event:CustomEvent) => {
    notifications.toast(event.detail?.msg ?? event.detail.value);
});

const tickets = [];
document.body.addEventListener("htmx:beforeRequest", (e) => {
    console.log("starting htmx request");
    const ticket = env.startLoading();
    tickets.push(ticket);
});
document.body.addEventListener("htmx:afterRequest", (e) => {
    console.log("done with request");
    for (let i = tickets.length-1; i >= 0; i--) {
        env.stopLoading(tickets[i]);
    }
});
