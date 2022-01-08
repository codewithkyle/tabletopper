import db from "@codewithkyle/jsql";

(async () => {
    // @ts-ignore
    const { env } = await import("/config.js");
    if (env === "production"){
        await navigator.serviceWorker.register('service-worker.js');
    }

    // @ts-ignore
    await db.start();

    //@ts-ignore
    await import("/js/routes.js");
})();