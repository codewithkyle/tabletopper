import { configure, mount } from "@codewithkyle/router";

(() => {
    document.body.innerHTML = "";
    const main = document.createElement("main");
    document.body.appendChild(main);
    mount(main);

    configure({
        "/": "home-page",
        "/room/{CODE}": "tabletop-page",
        "404": "missing-page",
    });
})();