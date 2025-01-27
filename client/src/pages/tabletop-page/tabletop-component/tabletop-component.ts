import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import Pawn from "~components/pawn/pawn";
import room from "room";
import TableCanvas from "./table-canvas/table-canvas";
import VFXCanvas from "./vfx-canvas/vfx-canvas";
import DoodleCanvas from "./doodle-canvas/doodle-canvas";

interface ITabletopComponent {
    map: string,
    players: Array<string>,
}
export default class TabeltopComponent extends SuperComponent<ITabletopComponent> {
    private moving: boolean;
    private measuring: boolean;
    public x: number;
    public y: number;
    private lastX: number;
    private lastY: number;
    public zoom: number;
    private canvas: TableCanvas;
    private vfxCanvas: VFXCanvas;
    private doodleCanvas: DoodleCanvas;
    private img: string;
    private isNewImage: boolean;
    private mode: "move" | "measure" | "lock";
    private startPos: Array<number>;
    private measurementEl: HTMLElement;

    constructor() {
        super();
        this.img = null;
        this.canvas = new TableCanvas();
        this.vfxCanvas = new VFXCanvas();
        this.doodleCanvas = new DoodleCanvas();
        this.moving = false;
        this.measuring = false;
        this.x = window.innerWidth * 0.5;
        this.y = (window.innerHeight - 28) * 0.5;
        this.zoom = 1;
        this.mode = "move";
        this.model = {
            map: null,
            players: [],
        };
        this.measurementEl = null;
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:fog:init":
                this.isNewImage = true;
                break;
            case "room:tabletop:pawn:delete":
                {
                    const pawn = this.querySelector(`pawn-component[data-uid="${data}"]`);
                    if (pawn) {
                        pawn.remove();
                    }
                }
                break;
            case "room:tabletop:pawn:spawn":
                {
                    const pawn = this.querySelector(`pawn-component[data-uid="${data.uid}"]`);
                    if (!pawn) {
                        const pawn = new Pawn(data);
                        this.appendChild(pawn);
                    }
                }
                break;
            case "room:tabletop:load":
                this.model.map = `https://tabletopper.nyc3.digitaloceanspaces.com/maps/${data}`;
                this.render();
                break;
            case "room:tabletop:clear":
                this.model.map = null;
                this.render();
                break;
        }
    }

    private locatePawn(el: HTMLElement) {
        if (el != null && el instanceof HTMLElement && el.isConnected) {
            this.moving = false;
            const bounds = el.getBoundingClientRect();
            const diffX = window.innerWidth * 0.5 - bounds.x;
            const diffY = window.innerHeight * 0.5 - bounds.y;
            this.x = this.x + diffX;
            this.y = this.y + diffY;
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
        }
    }

    override async connected() {
        await env.css(["tabletop-component"]);
        document.body.addEventListener("mousemove", this.handleMouseMove, { passive: true });
        document.body.addEventListener("mousedown", this.handleWindowDown, { passive: true });
        document.body.addEventListener("mouseup", this.handleMouseUp, { passive: true });
        document.body.addEventListener("touchmove", this.handleMouseMove, { passive: true });
        document.body.addEventListener("touchstart", this.handleWindowDown, { passive: true });
        document.body.addEventListener("touchend", this.handleMouseUp, { passive: true });
        window.addEventListener("wheel", this.handleScroll, { passive: true });
        document.body.addEventListener("drop", this.handleDrop, { passive: false, capture: true });
        window.addEventListener("tabletop:center:player", () => {
            const playerPawn = document.body.querySelector(`pawn-component[data-uid="${room.uid}"]`) as HTMLElement;
            this.locatePawn(playerPawn);
        });
        window.addEventListener("tabletop:view:reset", () => {
            this.moving = false;
            this.x = window.innerWidth * 0.5;
            this.y = (window.innerHeight - 28) * 0.5;
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
        });
        window.addEventListener("tabletop:view:zoom", (e: CustomEvent) => {
            const { zoom } = e.detail;
            if (zoom) {
                this.doZoom({ zoom: zoom, x: null, y: null, ratio: null });
            }
        });
        window.addEventListener("tabletop:view:zoom:in", () => {
            this.doZoom({ zoom: this.zoom + 0.1, x: null, y: null, ratio: null });
        });
        window.addEventListener("tabletop:view:zoom:out", () => {
            this.doZoom({ zoom: this.zoom - 0.1, x: null, y: null, ratio: null });
        });
        window.addEventListener("tabletop:mode", (e: CustomEvent) => {
            const { mode } = e.detail;
            this.moving = false;
            this.mode = mode;
            if (mode === "measure") {
                publish("tabletop", "cursor:measure");
            } else if (mode === "move") {
                publish("tabletop", "cursor:move");
            }
        });
        window.addEventListener("cursor:measure", (e: CustomEvent) => {
            publish("tabletop", "cursor:measure");
        });
        window.addEventListener("cursor:move", (e: CustomEvent) => {
            publish("tabletop", "cursor:move");
        });
        this.render();
    }

    private handleDrop: EventListener = (e: DragEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
    }

    private handleScroll: EventListener = (e: WheelEvent) => {
        if (!e.target.closest("tabletop-page")) return;
        let delta = e.deltaY;
        let sign = Math.sign(delta);
        let deltaAdjustedSpeed = Math.min(0.25, Math.abs(0.25 * delta / 128));
        let multiplier = 1 - sign * deltaAdjustedSpeed;
        let zoom = this.zoom * multiplier;
        const data = {
            zoom: zoom,
            x: e.clientX,
            y: e.clientY,
            delta: e.deltaY,
            ratio: multiplier,
        };
        this.moving = false;
        if (this.zoom !== data.zoom) {
            this.doZoom(data);
        }
    }

    private doZoom(data) {
        if (data.x === null) {
            data.x = window.innerWidth * .5;
        }
        if (data.y === null) {
            data.y = window.innerHeight * .5;
        }
        if (!data?.ratio) {
            if (data.zoom < this.zoom) {
                data.ratio = 0.8046875;
            }
            else if (data.zoom > this.zoom) {
                data.ratio = 1.1953125;
            }
            else {
                data.ratio = 0;
            }
        }
        this.zoom = data.zoom;
        if (this.zoom > 2) {
            this.zoom = 2;
        } else if (this.zoom < 0.1) {
            this.zoom = 0.1;
        }
        this.x = data.x - data.ratio * (data.x - this.x);
        this.y = data.y - data.ratio * (data.y - this.y);
        this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
        sessionStorage.setItem("zoom", this.zoom.toFixed(2).toString());
    }

    private handleWindowDown: EventListener = (e: MouseEvent | TouchEvent) => {
        if (!e.target.closest("tabletop-page")) return;
        else if (e instanceof MouseEvent && this.mode === "lock" && e.button != 1) return;
        if (window.TouchEvent && e instanceof TouchEvent) {
            this.lastX = e.touches[0].clientX;
            this.lastY = e.touches[0].clientY;
        } else if (e instanceof MouseEvent) {
            this.lastX = e.clientX;
            this.lastY = e.clientY;
        } else {
            return;
        }
        if (this.mode === "measure") {
            this.moving = e?.button === 1 ?? false;
            this.measuring = e?.button === 0 ?? true;
            this.startPos = [this.lastX, this.lastY];
        } else if (this.mode === "move") {
            this.moving = true;
        } else if (this.mode === "lock") {
            this.moving = true;
        }
    }

    private handleMouseUp: EventListener = () => {
        this.moving = false;
        this.measuring = false;
        if (this.measurementEl) {
            this.measurementEl?.remove();
            this.measurementEl = null;
        }
    }

    private handleMouseMove: EventListener = (e: MouseEvent | TouchEvent) => {
        let x = 0;
        let y = 0;
        if (window.TouchEvent && e instanceof TouchEvent) {
            x = e.touches[0].clientX;
            y = e.touches[0].clientY;
        } else {
            // @ts-ignore
            x = e.clientX;
            // @ts-ignore
            y = e.clientY;
        }
        if (this.moving) {
            const deltaX = this.lastX - x;
            const deltaY = this.lastY - y;
            this.x -= deltaX;
            this.y -= deltaY;
            this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
            this.lastX = x;
            this.lastY = y;
        }
        else if (this.measuring) {
            const currentPos = [x, y];
            if (!this.measurementEl) {
                this.measurementEl = document.createElement("div");
                this.measurementEl.className = "measurement";
                document.body.appendChild(this.measurementEl);
            }
            const distance = Math.sqrt(Math.pow(this.startPos[0] - currentPos[0], 2) + Math.pow(this.startPos[1] - currentPos[1], 2));

            const tabletopStartPos = this.convertViewportToTabletopPosition(this.startPos[0], this.startPos[1]);
            const tabletopCurrentPos = this.convertViewportToTabletopPosition(currentPos[0], currentPos[1]);
            const tabletopDistance = Math.sqrt(Math.pow(tabletopStartPos[0] - tabletopCurrentPos[0], 2) + Math.pow(tabletopStartPos[1] - tabletopCurrentPos[1], 2));

            this.measurementEl.style.width = `${distance}px`;

            let span = this.measurementEl.querySelector("span");
            if (!span) {
                span = document.createElement("span");
                this.measurementEl.appendChild(span);
            }
            span.innerText = `${Math.round(tabletopDistance / (room.gridSize)) * room.cellDistance}ft`;

            const rotate = `${Math.atan2(currentPos[1] - this.startPos[1], currentPos[0] - this.startPos[0]) * 180 / Math.PI}deg`;
            this.measurementEl.style.transform = `matrix(1, 0, 0, 1, ${this.startPos[0]}, ${this.startPos[1]}) rotate(${rotate})`;
            if (this.startPos[0] > currentPos[0]) {
                this.measurementEl.classList.add("left");
            } else {
                this.measurementEl.classList.remove("left");
            }
        }
    }

    public convertViewportToTabletopPosition(clientX: number, clientY: number): Array<number> {
        const bounds = this.getBoundingClientRect();
        const x = Math.round(clientX - bounds.left + this.scrollLeft) / this.zoom;
        const y = Math.round(clientY - bounds.top + this.scrollTop) / this.zoom;
        return [x, y];
    }

    override async render() {
        if (this.model.map) {
            this.img = this.model.map;
        } else {
            this.img = null;
            this.canvas.load(null);
        }
        if (!this.canvas?.isConnected) {
            this.parentElement.appendChild(this.canvas);
        }
        if (!this.vfxCanvas?.isConnected) {
            this.appendChild(this.vfxCanvas);
        }
        if (!this.doodleCanvas?.isConnected) {
            this.appendChild(this.doodleCanvas);
        }
        if (this.img) {
            this.canvas.load(this.img);
            //this.vfxCanvas.render(this.img);
            //this.doodleCanvas.render(this.img);
            this.x = window.innerWidth * 0.5;
            this.y = (window.innerHeight - 28) * 0.5;
        }
        this.style.transform = `matrix(${this.zoom}, 0, 0, ${this.zoom}, ${this.x}, ${this.y})`;
    }
}
env.bind("tabletop-component", TabeltopComponent);
