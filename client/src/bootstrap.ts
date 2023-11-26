import env from "~brixi/controllers/env";
import alerts from "~brixi/controllers/alerts";

(async () => {
    // @ts-ignore
    const { ENV } = await import("/config.js");
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
                    await import("/service-worker-assets.js?t=" + Date.now());
                    // @ts-expect-error
                    if (self.manifest.version !== localStorage.getItem("version")) {
                        update = true;
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
    // @ts-expect-error
    import("/js/tooltipper.js");
    env.css([
        "tooltip",
        "skeletons",
        "animations",
        "snackbar",
        "buttons",
        "toast",
        "notifications",
    ]);

    await db.start();

    await Promise.all([
        //db.query("RESET games"),
        db.query("RESET players"),
        db.query("RESET ledger"),
        db.query("RESET pawns"),
        db.query("RESET rolls"),
    ]);

    db.query("SELECT COUNT(index) FROM spells").then(results => {
        if (results[0]["COUNT(index)"] === 0){
            db.ingest(`${location.origin}/spells.ndjson`, "spells", "NDJSON");
        }
    });
    db.query("SELECT COUNT(index) FROM monsters").then(results => {
        if (results[0]["COUNT(index)"] === 0){
            db.ingest(`${location.origin}/monsters.ndjson`, "monsters", "NDJSON");
        }
    });

    connect();

    //@ts-ignore
    await import("/js/routes.js");
})();
