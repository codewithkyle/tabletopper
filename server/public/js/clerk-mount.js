const mount = document.querySelector("[data-clerk-mount]");
window.addEventListener("load", async () => {
    await window.Clerk.load({
        ui: { ClerkUI: window.__internal_ClerkUICtor },
    });
    if (window.Clerk.session) {
        location.href = "/authorize";
        return;
    }
    if (mount.dataset.clerkMount === "sign-up") {
        window.Clerk.mountSignUp(mount, { forceRedirectUrl: "/authorize" });
    } else {
        window.Clerk.mountSignIn(mount, { forceRedirectUrl: "/authorize" });
    }
});
