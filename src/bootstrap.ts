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

    connect();

    //@ts-ignore
    await import("/js/routes.js");
})();