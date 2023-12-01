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
    private brushCircle: HTMLElement;
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
        this.tabletop.addEventListener("mousedown", this.onMouseDown, { passive: false, capture: true });
        this.tabletop.addEventListener("mouseup", this.onMouseUp, { passive: false, capture: true });
        this.tabletop.addEventListener("mousemove", this.onMouseMove, { passive: false, capture: true });
        window.addEventListener("wheel", this.onMouseWheel, { passive: true, capture: true });
        this.render();
        this.brushCircle = document.createElement("brush-circle");
        document.body.appendChild(this.brushCircle);
    }

    disconnected(): void {
        if (this.brushCircle) {
            this.brushCircle.remove();
        }
        this.tabletop.removeEventListener("mousedown", this.onMouseDown, true);
        this.tabletop.removeEventListener("mouseup", this.onMouseUp, true);
        this.tabletop.removeEventListener("mousemove", this.onMouseMove, true);
        window.removeEventListener("wheel", this.onMouseWheel, true);
    }

    private onMouseDown = (e: MouseEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
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
        e.preventDefault();
        e.stopImmediatePropagation();
        this.painting = false;
        publish("doodle", {
            type: "stop",
            data: {
                x: e.clientX,
                y: e.clientY,
            },
        });
    }

    private updateBrushCircle(e){
        if (!this.brushCircle) return;
        let zoom = 1;
        if (sessionStorage.getItem("zoom")) {
            zoom = parseFloat(sessionStorage.getItem("zoom"));
        }
        if (this.mode === "erase") {
            this.brushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - 8}, ${e.clientY - 8})`;
        } else {
            this.brushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - 2}, ${e.clientY - 2})`;
        }
    }

    private onMouseWheel = (e: WheelEvent) => {
        this.updateBrushCircle(e);
    }

    private onMouseMove = (e: MouseEvent) => {
        this.updateBrushCircle(e);
        if (this.painting) {
            e.preventDefault();
            e.stopImmediatePropagation();
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
            e.preventDefault();
            e.stopImmediatePropagation();
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
                    if (this.mode === "erase") {
                        this.brushCircle.style.width = "16px";
                        this.brushCircle.style.height = "16px";
                    } else {
                        this.brushCircle.style.width = "4px";
                        this.brushCircle.style.height = "4px";
                    }
                    this.render();
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
