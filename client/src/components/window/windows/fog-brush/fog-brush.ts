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
    private polyPreviewEl: SVGElement;
    private mouse: Point;

    constructor() {
        super();
        this.painting = false;
        this.mode = "rect";
        this.points = [];
        this.mouse = { x: 0, y: 0 };
        this.rectPreviewEl = document.createElement("rect-preview");
        this.polyPreviewEl = document.createElementNS('http://www.w3.org/2000/svg', "svg");
        this.polyPreviewEl.classList.add("poly-preview");
        this.polyPreviewEl.setAttribute("viewBox", `0 0, ${window.innerWidth} ${window.innerHeight}`);
    }

    override async connected() {
        await env.css(["fog-brush"]);
        this.tabletop = document.querySelector("tabletop-page") as TabletopPage;
        this.tabletop.addEventListener("mousedown", this.onMouseDown);
        this.tabletop.addEventListener("mouseup", this.onMouseUp);
        this.tabletop.addEventListener("mousemove", this.onMouseMove);
        window.addEventListener("keydown", this.onKeyDown);
        this.render();
        publish("tabletop", "cursor:draw");
    }

    disconnected(): void {
        this.tabletop.removeEventListener("mousedown", this.onMouseDown);
        this.tabletop.removeEventListener("mouseup", this.onMouseUp);
        this.tabletop.removeEventListener("mousemove", this.onMouseMove);
        window.removeEventListener("keydown", this.onKeyDown);
        this.rectPreviewEl.remove();
        this.polyPreviewEl.remove();
        publish("tabletop", "cursor:move");
    }

    private updateRectPreview(x: number, y: number) {
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

    private updatePolyPreview() {
        if (!this.painting) {
            this.polyPreviewEl.classList.add("hidden");
        }
        if (this.mode === "poly") {
            if (!this.polyPreviewEl?.isConnected) {
                document.body.appendChild(this.polyPreviewEl);
            }
            this.polyPreviewEl.innerHTML = "";
            this.polyPreviewEl.classList.remove("hidden");
            for (let i = 0; i < this.points.length - 1; i++) {
                const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
                line.setAttribute('x1', this.points[i].x.toString());
                line.setAttribute('y1', this.points[i].y.toString());
                line.setAttribute('x2', this.points[i + 1].x.toString());
                line.setAttribute('y2', this.points[i + 1].y.toString());
                line.setAttribute('stroke', 'yellow');
                line.setAttribute('stroke-width', "1");
                this.polyPreviewEl.appendChild(line);
            }
            const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
            line.setAttribute('x1', this.points[this.points.length-1].x.toString());
            line.setAttribute('y1', this.points[this.points.length-1].y.toString());
            line.setAttribute('x2', this.mouse.x.toString());
            line.setAttribute('y2', this.mouse.y.toString());
            line.setAttribute('stroke', 'yellow');
            line.setAttribute('stroke-width', "1");
            this.polyPreviewEl.appendChild(line);
            console.log("line", line);
        }
    }

    private onKeyDown = (e: KeyboardEvent) => {
        const key = e.key.toLowerCase();
        if (key === "enter" && this.painting && this.mode === "poly"){
            this.polyPreviewEl.classList.add("hidden");
            if (this.points.length >= 2) {
                publish("fog", {
                    type: "poly",
                    points: this.points,
                });
            }
            this.points = [];
            this.painting = false;
        }
    }

    private onMouseDown = (e: MouseEvent) => {
        if (e.button === 0) {
            this.painting = true;
            const x = e.clientX;
            const y = e.clientY;
            this.points.push({ x, y });
            if (this.mode === "poly") {
                this.updatePolyPreview();
            }
        }
    }

    private onMouseUp = (e: MouseEvent) => {
        const x = e.clientX;
        const y = e.clientY;
        if (this.painting) {
            if (this.mode === "rect") {
                this.rectPreviewEl.classList.add("hidden");
                this.points.push({ x, y });
                if (this.points.length === 2) {
                    publish("fog", {
                        type: "rect",
                        points: this.points,
                    });
                }
                this.points = [];
                this.painting = false;
            }
        }
    }

    private onMouseMove = (e: MouseEvent) => {
        const x = e.clientX;
        const y = e.clientY;
        this.mouse = { x, y };
        if (this.painting) {
            switch(this.mode){
                case "rect":
                    this.updateRectPreview(x, y);
                    break;
                case "poly":
                    this.updatePolyPreview();
                    break;
            }
        }
    }

    render() {
        const view = html`
            <group-button-component
                class="mb-0.5"
                data-buttons='[{"label":"Rectangle","id":"rect"},{"label":"Polygon","id":"poly"}]'
                data-active="${this.mode}"
                @change=${(e) => {
                    this.mode = e.detail.id;
                    this.render();
                }}
            ></group-button-component>
            ${this.mode === "poly" ? html`<p class="font-xs px-0.5">Press 'enter' to complete path.</p>` : ""}
        `;
        render(view, this);
    }
}
env.bind("fog-brush", FogBrush);
