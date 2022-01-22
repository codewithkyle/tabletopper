import db from "@codewithkyle/jsql";
import env from "~brixi/controllers/env";
import { connect } from "~controllers/ws";

(async () => {
    // @ts-ignore
    const { ENV } = await import("/config.js");
    if (ENV === "production"){
        await navigator.serviceWorker.register('service-worker.js');
    }

    // @ts-expect-error
    import("/js/soundscape.js");
    // @ts-expect-error
    import("/js/tooltipper.js");
    env.css([
        "tooltip",
        "skeletons",
        "toast",
        "animations",
        "snackbar",
        "modals",
        "modal"
    ]);

    await db.start({
        dbWorker: `${location.origin}/jsql.worker.js`,
        streamWorker: `${location.origin}/stream.worker.js`,
    });

    await Promise.all([
        db.query("RESET games"),
        db.query("RESET players"),
        db.query("RESET ledger"),
    ]);

    db.query("SELECT COUNT(index) FROM spells").then(results => {
        if (results[0] === 0){
            db.ingest(`${location.origin}/spells.ndjson`, "spells", "NDJSON");
        }
    });
    db.query("SELECT COUNT(index) FROM monsters").then(results => {
        if (results[0] === 0){
            db.ingest(`${location.origin}/monsters.ndjson`, "monsters", "NDJSON");
        }
    });

    connect();

    //@ts-ignore
    await import("/js/routes.js");
})();