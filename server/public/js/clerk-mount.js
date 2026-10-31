const mount = document.querySelector("[data-clerk-mount]");
function authorize(token) {
    const form = document.createElement("form");
    form.method = "post";
    form.action = "/authorize";
    form.hidden = true;
    const field = document.createElement("input");
    field.type = "hidden";
    field.name = "token";
    field.value = token;
    form.append(field);
    document.body.append(form);
    form.submit();
}
window.addEventListener("load", async () => {
    await window.Clerk.load({
        ui: { ClerkUI: window.__internal_ClerkUICtor },
    });
    if (window.Clerk.session) {
        const token = await window.Clerk.session.getToken({ skipCache: true });
        if (token) {
            authorize(token);
            return;
        }
    }
    if (mount.dataset.clerkMount === "sign-up") {
        window.Clerk.mountSignUp(mount, {
            forceRedirectUrl: "/sign-in",
            signInUrl: "/sign-in",
        });
    } else {
        window.Clerk.mountSignIn(mount, {
            forceRedirectUrl: "/sign-in",
            signUpUrl: "/sign-up",
        });
    }
});
