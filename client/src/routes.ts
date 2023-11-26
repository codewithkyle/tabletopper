import { router, mount } from "@codewithkyle/router";

(() => {
    // Begin router
    router.add("/", "home-page");
    router.add("/room/{CODE}", "tabletop-page");
    router.add("*", "missing-page");

    document.body.innerHTML = "";
    const main = document.createElement("main");
    document.body.appendChild(main);
    mount(main);
})();
