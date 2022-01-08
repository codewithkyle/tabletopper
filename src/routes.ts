import { configure, mount } from "@codewithkyle/router";

(() => {
    const main = document.body;
    mount(main);

    configure({
        "/": "home-page",
        "404": "missing-page",
    });
})();