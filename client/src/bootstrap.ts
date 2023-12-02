import alerts from "~brixi/controllers/alerts";

(async () => {
    // @ts-ignore
    const { ENV } = await import("/static/config.js");
    if (ENV === "production"){
        let update = false;
        await new Promise<void>(resolve => {
            navigator.serviceWorker.register("/service-worker.js", { scope: "/" });
            navigator.serviceWorker.addEventListener("message", (e) => {
                localStorage.setItem("version", e.data);
                if (update){
                    alerts.snackbar("An update has been applied.", [
                        {
                            label: "Restart",
                            callback: ()=>{
                                location.reload();
                            },
                        }
                    ]);
                }
            });
            navigator.serviceWorker.ready.then(async (registration) => {
                try {
                    await import("/static/service-worker-assets.js?t=" + Date.now());
                    // @ts-expect-error
                    if (self.manifest.version !== localStorage.getItem("version")) {
                        update = true;
                        alerts.alert("Installing Update", "An app update is in progress. Please wait...");
                        localStorage.removeItem("version");
                    } else {
                        // @ts-expect-error
                        delete self.manifest.assets;
                    }
                    // @ts-expect-error
                    registration.active.postMessage(self.manifest);
                } catch (e) {
                    registration.active.postMessage({
                        version: localStorage.getItem("version") || "init",
                    });
                }
                resolve();
            });
        });
    }

    // @ts-expect-error
    import("/js/soundscape.js").then(module => {
        module.default.load();
    });
})();
