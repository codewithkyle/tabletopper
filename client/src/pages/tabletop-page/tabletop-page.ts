import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import { send } from "~controllers/ws";
import ContextMenu from  "~brixi/components/context-menu/context-menu";

interface ITabletopPage {
}
export default class TabletopPage extends SuperComponent<ITabletopPage>{
    private spotlightSearchEl: HTMLElement;

    constructor(){
        super();
        this.spotlightSearchEl = null;
    }

    override async connected(){
        this.setAttribute("state", "IDLING");
        this.render();
        window.addEventListener("keydown", this.handleKeyboard);
        this.addEventListener("contextmenu", this.handleContextMenu);
    }

    private async handleSpotlightCallback(type: "spell"|"monster", index:string = null){
        let window;
        switch(type){
            case "spell":
                break;
            case "monster":
                break;
            default:
                break;
        }
        if (window && !window.isConnected){
            document.body.appendChild(window);
        }
    }

    private handleContextMenu = (e) => {
        e.preventDefault();
        const x = e.clientX;
        const y = e.clientY;
        const items = [
            {
                label: "Ping",
                callback: ()=>{
                    const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                    let diffX = (x - tabletop.x) / tabletop.zoom;
                    let diffY = (y - tabletop.y) / tabletop.zoom;
                    send("room:tabletop:ping", {
                        x: Math.round(diffX) - 16,
                        y: Math.round(diffY) - 16,
                        color: sessionStorage.getItem("color"),
                    });
                }
            }
        ];
        new ContextMenu({
            x: x,
            y: y,
            items: items,
        });
    }

    private handleKeyboard = (e:KeyboardEvent) => {
        if (e?.["KeyStatus"]?.["RepeatCount"]){
            e["KeyStatus"]["RepeatCount"]++;
        } else {
            e["KeyStatus"] = {
                RepeatCount: 1,
            };
        }
        if (e instanceof KeyboardEvent){
            const key = e.key.toLowerCase();
            switch(key){
                case " ":
                    if (e.ctrlKey && (this.spotlightSearchEl === null || !this.spotlightSearchEl.isConnected) && e?.["KeyStatus"]?.["RepeatCount"] === 1){
                        this.spotlightSearchEl = new SpotlightSearch(sessionStorage.getItem("role") === "gm", true, this.handleSpotlightCallback.bind(this));
                        document.body.appendChild(this.spotlightSearchEl);
                    }
                    break;
                default:
                    break;
            }
        }
    }
}
env.bind("tabletop-page", TabletopPage);

