import db from "@codewithkyle/jsql";
import env from "~brixi/controllers/env";

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
    });

    //@ts-ignore
    await import("/js/routes.js");
})();