import env from "~brixi/controllers/env";
import Window from "~components/window/window";

export type Axis = "x" | "y" | "both";
export default class ResizeHandle extends HTMLElement{
    private resizing:boolean;
    private pos1:number;
    private pos2:number;
    private pos3:number;
    private pos4:number;
    private target:HTMLElement;
    private axis: Axis;

    constructor(target:HTMLElement, axis:Axis){
        super();
        this.resizing = false;
        this.pos1 = 0;
        this.pos2 = 0;
        this.pos3 = 0;
        this.pos4 = 0;
        this.target = target;
        this.axis = axis;
    }

    private handleMouseDown:EventListener = (e:MouseEvent|TouchEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        this.resizing = true;
        if (e instanceof MouseEvent){
            this.pos3 = e.clientX;
            this.pos4 = e.clientY;
        } else if (e instanceof TouchEvent){
            this.pos3 = e.touches[0].clientX;
            this.pos4 = e.touches[0].clientY;
        }
    }

    private handleMouseUp:EventListener = () => {
        if (this.resizing){
            // @ts-ignore
            this.target?.save();
        }
        this.resizing = false;
    }

    private handleMouseMove:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.resizing){
            if (e instanceof MouseEvent){
                this.pos1 = this.pos3 - e.clientX;
                this.pos2 = this.pos4 - e.clientY;
                this.pos3 = e.clientX;
                this.pos4 = e.clientY;
            }else if (e instanceof TouchEvent){
                this.pos1 = this.pos3 - e.touches[0].clientX;
                this.pos2 = this.pos4 - e.touches[0].clientY;
                this.pos3 = e.touches[0].clientX;
                this.pos4 = e.touches[0].clientY;
            }
            const bounds = this.target.getBoundingClientRect();
            let height = bounds.height - this.pos2;
            let width = bounds.width - this.pos1;
            if (height < 231){
                height = 231;
            } else if (height > window.innerHeight - 28){
                height = window.innerHeight - 28;
            }
            if (width < 411){
                width = 411;
            } else if (width > window.innerWidth){
                width = window.innerWidth;
            }
            switch (this.axis){
                case "x":
                    this.target.style.width = `${width}px`;
                    break;
                case "y":
                    this.target.style.height = `${height}px`;
                    break;
                default:
                    this.target.style.width = `${width}px`;
                    this.target.style.height = `${height}px`;
                    break;
            }
        }
    }

    connectedCallback(){        
        window.addEventListener("mouseup", this.handleMouseUp, { capture: true, passive: true });
        window.addEventListener("mousemove", this.handleMouseMove, { capture: true, passive: true });
        window.addEventListener("touchend", this.handleMouseUp, { capture: true, passive: true });
        window.addEventListener("touchmove", this.handleMouseMove, { capture: true, passive: true });
        this.addEventListener("mousedown", this.handleMouseDown);
        this.addEventListener("touchstart", this.handleMouseDown);
    }
}
env.bind("resize-handle", ResizeHandle);