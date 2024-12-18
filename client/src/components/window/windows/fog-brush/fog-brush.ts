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

    constructor() {
        super();
        this.painting = false;
        this.mode = "rect";
        this.points = [];
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
        publish("tabletop", "cursor:move");
    }

    private onMouseDown = (e: MouseEvent) => {
        if (e.button === 0) {
            this.painting = true;
            const x = e.clientX;
            const y = e.clientY;
            this.points.push({ x, y});
            console.log("fog clear start point", x, y);
        }
    }

    private onMouseUp = (e: MouseEvent) => {
        const x = e.clientX;
        const y = e.clientY;
        if (this.painting) {
            if (this.mode === "rect") {
                console.log("fog clear end point", x, y);
                this.points.push({ x, y });
                if (this.points.length === 2){
                    console.log("has rect points", this.points);
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
