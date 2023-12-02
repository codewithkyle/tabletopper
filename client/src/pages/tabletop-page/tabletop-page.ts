import SuperComponent from "@codewithkyle/supercomponent";
import "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import { send } from "~controllers/ws";
import ContextMenu from "~brixi/components/context-menu/context-menu";
import TabeltopComponent from "./tabletop-component/tabletop-component";
import { subscribe } from "@codewithkyle/pubsub";
import PingComponent from "./tabletop-component/ping-component/ping-component";
import room from "room";

declare const htmx: any;

interface ITabletopPage {
}
export default class TabletopPage extends SuperComponent<ITabletopPage>{

    override async connected() {
        this.addEventListener("contextmenu", this.handleContextMenu);
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:ping":
                const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                const el = new PingComponent(data.x, data.y, data.color);
                tabletop.appendChild(el);
                break;
            default:
                break;
        }
    }

    private handleContextMenu = (e) => {
        e.preventDefault();
        const x = e.clientX;
        const y = e.clientY;
        document.body.querySelectorAll("context-menu").forEach((el) => {
            el.remove();
        });
        const items = [
            {
                label: "Ping",
                callback: () => {
                    const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    send("room:tabletop:ping", {
                        x: Math.round(diffX) - 16,
                        y: Math.round(diffY) - 16,
                        color: "yellow",
                    });
                }
            }
        ];
        if (room.isGM) {
            items.push({
                label: "Spawn Monster",
                callback: () => {
                    const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    sessionStorage.setItem("tabletop:spawn-monster:y", `${Math.round(diffY) - 16}`);
                    sessionStorage.setItem("tabletop:spawn-monster:x", `${Math.round(diffX) - 16}`);
                    htmx.ajax("GET", "/stub/tabletop/spotlight", { target: "spotlight-search .modal" });
                    window.dispatchEvent(new CustomEvent("show-spotlight-search"));
                }
            });
            items.push({
                label: "Quick Spawn",
                callback: () => {
                    const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    sessionStorage.setItem("tabletop:spawn-monster:y", `${Math.round(diffY) - 16}`);
                    sessionStorage.setItem("tabletop:spawn-monster:x", `${Math.round(diffX) - 16}`);
                    window.dispatchEvent(new CustomEvent("show-quick-spawn"));
                }
            });
        }
        new ContextMenu({
            x: x,
            y: y,
            items: items,
        });
    }
}
env.bind("tabletop-page", TabletopPage);

