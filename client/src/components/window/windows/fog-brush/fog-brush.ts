import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import "~brixi/components/buttons/group-button/group-button";
import { publish } from "@codewithkyle/pubsub";
import room from "room";
import TabletopPage from "pages/tabletop-page/tabletop-page";

interface IFogBrush { }
export default class FogBrush extends SuperComponent<IFogBrush>{
    private painting: boolean;
    private mode: "eraser" | "fill";
    private brushSize: number;
    private fogBrushCircle: HTMLElement;
    private tabletop: TabletopPage;

    constructor() {
        super();
        this.painting = false;
        this.mode = "eraser";
        this.brushSize = 2;
    }

    override async connected() {
        await env.css(["fog-brush"]);
        this.tabletop = document.querySelector("tabletop-page") as TabletopPage;
        this.tabletop.addEventListener("mousedown", this.onMouseDown);
        this.tabletop.addEventListener("mouseup", this.onMouseUp);
        this.tabletop.addEventListener("mousemove", this.onMouseMove);
        window.addEventListener("wheel", this.onMouseWheel, { passive: true });
        this.render();
        this.fogBrushCircle = document.createElement("fog-brush-circle");
        document.body.appendChild(this.fogBrushCircle);
        this.scaleBrush();
        publish("tabletop", "cursor:draw");
    }

    disconnected(): void {
        if (this.fogBrushCircle) {
            this.fogBrushCircle.remove();
        }
        this.tabletop.removeEventListener("mousedown", this.onMouseDown);
        this.tabletop.removeEventListener("mouseup", this.onMouseUp);
        this.tabletop.removeEventListener("mousemove", this.onMouseMove);
        window.removeEventListener("wheel", this.onMouseWheel);
        publish("tabletop", "cursor:move");
    }

    private scaleBrush() {
        if (this.fogBrushCircle) {
            if (this.brushSize > 2) {
                this.fogBrushCircle.style.width = `${(this.brushSize - 2) * room.gridSize}px`;
                this.fogBrushCircle.style.height = `${(this.brushSize - 2) * room.gridSize}px`;
            } else {
                this.fogBrushCircle.style.width = `${(this.brushSize - 1) * room.gridSize}px`;
                this.fogBrushCircle.style.height = `${(this.brushSize - 1) * room.gridSize}px`;
            }
        }
    }

    private onMouseDown = (e: MouseEvent) => {
        if (e.button === 0) {
            this.painting = true;
            const x = e.clientX;
            const y = e.clientY;
            publish("fog", {
                type: this.mode,
                data: {
                    x: x,
                    y: y,
                    brushSize: this.brushSize,
                },
            });
        }
    }

    private onMouseUp = (e: MouseEvent) => {
        this.painting = false;
    }

    private onMouseWheel = (e: WheelEvent) => {
        if (this.fogBrushCircle) {
            let zoom = 1;
            if (sessionStorage.getItem("zoom")) {
                zoom = parseFloat(sessionStorage.getItem("zoom"));
            }
            if (this.brushSize > 2) {
                this.fogBrushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - (room.gridSize * (this.brushSize - 2) * 0.5)}, ${e.clientY - (room.gridSize * (this.brushSize - 2) * 0.5)})`;
            } else {
                this.fogBrushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - (room.gridSize * (this.brushSize - 1) * 0.5)}, ${e.clientY - (room.gridSize * (this.brushSize - 1) * 0.5)})`;
            }
        }
    }

    private onMouseMove = (e: MouseEvent) => {
        if (this.fogBrushCircle) {
            let zoom = 1;
            if (sessionStorage.getItem("zoom")) {
                zoom = parseFloat(sessionStorage.getItem("zoom"));
            }
            if (this.brushSize > 2) {
                this.fogBrushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - (room.gridSize * (this.brushSize - 2) * 0.5)}, ${e.clientY - (room.gridSize * (this.brushSize - 2) * 0.5)})`;
            } else {
                this.fogBrushCircle.style.transform = `matrix(${zoom}, 0, 0, ${zoom}, ${e.clientX - (room.gridSize * (this.brushSize - 1) * 0.5)}, ${e.clientY - (room.gridSize * (this.brushSize - 1) * 0.5)})`;
            }
        }
        if (this.painting) {
            const x = e.clientX;
            const y = e.clientY;
            publish("fog", {
                type: this.mode,
                data: {
                    x: x,
                    y: y,
                    brushSize: this.brushSize,
                },
            });
        }
    }

    render() {
        const view = html`
            <group-button-component
                class="mb-1"
                data-buttons='[{"label":"Eraser","id":"eraser"},{"label":"Fill","id":"fill"}]'
                data-active="${this.mode}"
                @change=${(e) => {
                this.mode = e.detail.id;
            }}
            ></group-button-component>
            <select-component 
                data-label="Brush Size" 
                data-options='[{"label":"Small","value":"2"},{"label":"Medium","value":"4"},{"label":"Large","value":"8"}]' 
                data-value="${this.brushSize}"
                @change=${(e) => {
                this.brushSize = parseInt(e.detail.value);
                this.scaleBrush();
            }}
            ></select-component>
        `;
        render(view, this);
    }
}
env.bind("fog-brush", FogBrush);
