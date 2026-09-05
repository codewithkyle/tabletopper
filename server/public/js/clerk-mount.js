// Mounts Clerk's hosted sign-in or sign-up UI into the element carrying
// data-clerk-mount. clerk.templ renders that element and the Clerk script
// tag beside it, with the publishable key and script URL from the server's
// config; nothing here knows which environment it is in.
//
// The `load` event is the right moment: Clerk's script is async, and load
// fires only once every async script has run.
const mount = document.querySelector("[data-clerk-mount]");

window.addEventListener("load", async () => {
    await window.Clerk.load();

    // Already signed in with Clerk -- clerk-js has just set the __session
    // cookie /authorize reads, so hand over rather than show a form.
    if (window.Clerk.session) {
        location.href = "/authorize";
        return;
    }

    const options = {
        appearance: {
            baseTheme: matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
        },
        afterSignInUrl: "/authorize",
        afterSignUpUrl: "/authorize",
    };
    if (mount.dataset.clerkMount === "sign-up") {
        window.Clerk.mountSignUp(mount, options);
    } else {
        window.Clerk.mountSignIn(mount, options);
    }
});
