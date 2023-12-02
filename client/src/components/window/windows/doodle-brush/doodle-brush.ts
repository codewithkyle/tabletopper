import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import "~brixi/components/buttons/group-button/group-button";
import { publish } from "@codewithkyle/pubsub";
import TabletopPage from "pages/tabletop-page/tabletop-page";

interface IDoodleBrush { }
export default class DoodleBrush extends SuperComponent<IDoodleBrush>{
    private painting: boolean;
    private mode: "draw" | "erase";
    private tabletop: TabletopPage;
    private color: string;

    constructor() {
        super();
        this.painting = false;
        this.mode = "draw";
        this.color = "#ffffff";
    }

    override async connected() {
        await env.css(["doodle-brush"]);
        this.tabletop = document.querySelector("tabletop-page") as TabletopPage;
        this.tabletop.addEventListener("mousedown", this.onMouseDown);
        this.tabletop.addEventListener("mouseup", this.onMouseUp);
        this.tabletop.addEventListener("mousemove", this.onMouseMove);
        this.render();
        publish("tabletop", "cursor:draw");
    }

    disconnected(): void {
        this.tabletop.removeEventListener("mousedown", this.onMouseDown);
        this.tabletop.removeEventListener("mouseup", this.onMouseUp);
        this.tabletop.removeEventListener("mousemove", this.onMouseMove);
        publish("tabletop", "cursor:move");
    }

    private onMouseDown = (e: MouseEvent) => {
        if (e.button === 0) {
            this.painting = true;
            const x = e.clientX;
            const y = e.clientY;
            publish("doodle", {
                type: this.mode,
                data: {
                    x: x,
                    y: y,
                    color: this.color,
                },
            });
        }
    }

    private onMouseUp = (e: MouseEvent) => {
        this.painting = false;
        publish("doodle", {
            type: "stop",
            data: {
                x: e.clientX,
                y: e.clientY,
            },
        });
    }

    private onMouseMove = (e: MouseEvent) => {
        if (this.painting) {
            const x = e.clientX;
            const y = e.clientY;
            publish("doodle", {
                type: this.mode,
                data: {
                    x: x,
                    y: y,
                    color: this.color,
                },
            });
        } else {
            const x = e.clientX;
            const y = e.clientY;
            publish("doodle", {
                type: "mouse",
                data: {
                    x: x,
                    y: y,
                },
            });
        }
    }

    render() {
        const view = html`
            <group-button-component
                class="mb-1"
                data-buttons='[{"label":"Eraser","id":"erase"},{"label":"Draw","id":"draw"}]'
                data-active="${this.mode}"
                @change=${(e) => {
                    this.mode = e.detail.id;
                }}
            ></group-button-component>
            ${ this.mode === "erase" ? "" : html`
                <select-component 
                    data-label="Color" 
                    data-options='[{"label":"White","value":"#ffffff"},{"label":"Blue","value":"#3b82f6"},{"label":"Green","value":"#22c55e"},{"label":"Orange","value":"#f97316"},{"label":"Purple","value":"#a855f7"},{"label":"Pink","value":"#ec4899"},{"label":"Red","value":"#ef4444"},{"label":"Yellow","value":"#eab308"}]' 
                    data-value="#ffffff"
                    @change=${(e) => {
                        this.color = e.detail.value;
                        publish("doodle", {
                            type: "color",
                            data: this.color,
                        });
                    }}
                ></select-component>
            `}
        `;
        render(view, this);
    }
}
env.bind("doodle-brush", DoodleBrush);
