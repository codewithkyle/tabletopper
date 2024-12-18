import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import "~brixi/components/buttons/group-button/group-button";
import { publish } from "@codewithkyle/pubsub";
import room from "room";
import TabletopPage from "pages/tabletop-page/tabletop-page";

type Point = {
    x: number,
    y: number,
}
interface IFogBrush { }
export default class FogBrush extends SuperComponent<IFogBrush> {
    private painting: boolean;
    private tabletop: TabletopPage;
    private mode: "rect" | "poly";
    private points: Array<Point>;
    private rectPreviewEl: HTMLElement;

    constructor() {
        super();
        this.painting = false;
        this.mode = "rect";
        this.points = [];
        this.rectPreviewEl = document.createElement("rect-preview");
    }

    override async connected() {
        await env.css(["fog-brush"]);
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
        this.rectPreviewEl.remove();
        publish("tabletop", "cursor:move");
    }

    private updateRectPreview(x:number, y:number){
        if (!this.painting) {
            this.rectPreviewEl.classList.add("hidden");
        }
        if (this.mode === "rect") {
            if (!this.rectPreviewEl?.isConnected) {
                document.body.appendChild(this.rectPreviewEl);
            }
            this.rectPreviewEl.classList.remove("hidden");
            const width = x - this.points[0].x;
            const height = y - this.points[0].y;
            this.rectPreviewEl.style.top = `${this.points[0].y}px`;
            this.rectPreviewEl.style.left = `${this.points[0].x}px`;
            this.rectPreviewEl.style.width = `${width}px`;
            this.rectPreviewEl.style.height = `${height}px`;
        }
    }

    private onMouseDown = (e: MouseEvent) => {
        if (e.button === 0) {
            this.painting = true;
            const x = e.clientX;
            const y = e.clientY;
            this.points.push({ x, y});
        }
    }

    private onMouseUp = (e: MouseEvent) => {
        const x = e.clientX;
        const y = e.clientY;
        if (this.painting) {
            if (this.mode === "rect") {
                this.rectPreviewEl.classList.add("hidden");
                this.points.push({ x, y });
                if (this.points.length === 2){
                    publish("fog", {
                        type: "rect",
                        points: this.points,
                    });
                }
            }
        }
        this.points = [];
        this.painting = false;
    }

    private onMouseMove = (e: MouseEvent) => {
        if (this.painting) {
            const x = e.clientX;
            const y = e.clientY;
            if (this.mode === "rect") {
                this.updateRectPreview(x, y);
            }
        }
    }

    render() {
        const view = html`
            <group-button-component
                class="mb-1"
                data-buttons='[{"label":"Rectangle","id":"rect"},{"label":"Polygon","id":"poly"}]'
                data-active="${this.mode}"
                @change=${(e) => {
                    this.mode = e.detail.id;
                }}
            ></group-button-component>
        `;
        render(view, this);
    }
}
env.bind("fog-brush", FogBrush);
