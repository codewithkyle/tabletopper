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
        this.setAttribute("cursor", "move");
        this.addEventListener("contextmenu", this.handleContextMenu);
        window.addEventListener("beforeunload", function (e) {
            var confirmationMessage = 'Are you sure you want to exit?';
            (e || window.event).returnValue = confirmationMessage; //Gecko + IE
            return confirmationMessage; //Gecko + Webkit, Safari, Chrome etc.
        });
        subscribe("socket", this.inbox.bind(this));
        subscribe("tabletop", this.tabletopInbox.bind(this));
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

    private tabletopInbox(type) {
        switch (type) {
            case "cursor:move":
                this.setAttribute("cursor", "move");
                break;
            case "cursor:grab":
                this.setAttribute("cursor", "grab");
                break;
            case "cursor:grabbing":
                this.setAttribute("cursor", "grabbing");
                break;
            case "cursor:draw":
                this.setAttribute("cursor", "draw");
                break;
            case "cursor:measure":
                this.setAttribute("cursor", "draw");
                break;
            default:
                this.setAttribute("cursor", "move");
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
            },
            {
                label: "Quick Spawn",
                callback: () => {
                    const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    sessionStorage.setItem("tabletop:spawn-monster:y", `${Math.round(diffY) - 16}`);
                    sessionStorage.setItem("tabletop:spawn-monster:x", `${Math.round(diffX) - 16}`);
                    window.dispatchEvent(new CustomEvent("show-quick-spawn"));
                }
            },
        ];
        if (room.isGM) {
            items.push({
                label: "Spawn Monster",
                callback: () => {
                    window.dispatchEvent(new CustomEvent("show-monster-menu"));
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

