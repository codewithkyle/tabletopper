import notifications from "~brixi/controllers/alerts";

window.addEventListener("toast", (event:CustomEvent) => {
    notifications.toast(event.detail?.msg ?? event.detail.value);
});
