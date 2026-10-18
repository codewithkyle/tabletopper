const DISMISSED_KEY = "sunset-notice-dismissed";
const SNOOZE_MS = 7 * 24 * 60 * 60 * 1000;

function dismissedAt(): number {
    try {
        return Number(localStorage.getItem(DISMISSED_KEY)) || 0;
    } catch (e) {
        return 0;
    }
}

function rememberDismissal(): void {
    try {
        localStorage.setItem(DISMISSED_KEY, Date.now().toString());
    } catch (e) {}
}

const dialog = document.getElementById("sunset-notice") as HTMLDialogElement | null;

if (dialog && typeof dialog.showModal === "function" && Date.now() - dismissedAt() > SNOOZE_MS) {
    dialog.addEventListener("close", rememberDismissal);
    dialog.addEventListener("click", (e) => {
        if ((e.target as HTMLElement).closest("a")) {
            dialog.close();
        }
    });
    dialog.showModal();
}
