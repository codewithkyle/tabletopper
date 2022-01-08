import db from "@codewithkyle/jsql";

(async () => {
    // @ts-ignore
    const { ENV } = await import("/config.js");
    if (ENV === "production"){
        await navigator.serviceWorker.register('service-worker.js');
    }

    await db.start();

    //@ts-ignore
    await import("/js/routes.js");
})();