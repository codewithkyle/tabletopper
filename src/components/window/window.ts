import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import ResizeHandle from "~components/resize-handle/resize-handle";

interface IWindow {
    size: "minimized" | "maximized" | "normal";
};
export default class Window extends SuperComponent<IWindow>{
    private moving: boolean;
    private x: number;
    private y: number;
    private view: HTMLElement | TemplateResult | string;
    private name: string;
    private handle: string;
    private w: number;
    private h: number;
    private localX: number;
    private localY: number;

    constructor(name:string, view:HTMLElement | TemplateResult | string){
        super();
        this.handle = name.toLowerCase().trim().replace(/\s+/g, "-");

        const savedX = localStorage.getItem(`${this.handle}-x`);
        const savedY = localStorage.getItem(`${this.handle}-y`);
        this.x = savedX ? parseInt(savedX) : 0;
        this.y = savedY ? parseInt(savedY) : 28;
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        if (savedX == null){
            localStorage.setItem(`${this.handle}-x`, this.x.toFixed(0).toString());
        }
        if (savedY == null){
            localStorage.setItem(`${this.handle}-y`, this.y.toFixed(0).toString());
        }

        const savedWidth = localStorage.getItem(`${this.handle}-w`);
        const savedHeight = localStorage.getItem(`${this.handle}-h`);
        this.w = savedWidth ? parseInt(savedWidth) : 411;
        this.h = savedHeight ? parseInt(savedHeight) : 231;
        if (savedWidth == null){
            localStorage.setItem(`${this.handle}-w`, this.w.toFixed(0).toString());
        }
        if (savedHeight == null){
            localStorage.setItem(`${this.handle}-h`, this.h.toFixed(0).toString());
        }

        this.view = view;
        this.name = name;
        this.moving = false;
        this.model = {
            size: "normal",
        };
    }

    override async connected(){
        await env.css(["window"]);
        window.addEventListener("mouseup", this.stopMove, { capture: true, passive: true });
        window.addEventListener("mousemove", this.move, { capture: true, passive: true });
        this.render();
    }

    public maximize(){
        this.moving = false;
        this.x = 0;
        this.y = 0;
        this.w = window.innerWidth;
        this.h = window.innerHeight;
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        this.set({
            size: "maximized",
        });
    }

    public minimize(){
        this.moving = false;
        this.h = 28;
        this.w = parseInt(localStorage.getItem(`${this.handle}-w`));
        this.set({
            size: "minimized",
        });
    }

    public windowed(){
        this.moving = false;
        this.x = parseInt(localStorage.getItem(`${this.handle}-x`));
        this.y = parseInt(localStorage.getItem(`${this.handle}-y`));
        this.w = parseInt(localStorage.getItem(`${this.handle}-w`));
        this.h = parseInt(localStorage.getItem(`${this.handle}-h`));
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        this.set({
            size: "normal",
        });
    }
    
    public close(){
        this.remove();
    }

    public save(){
        const bounds = this.getBoundingClientRect();
        this.x = bounds.x;
        this.y = bounds.y;
        this.w = bounds.width;
        if (this.w < 411){
            this.w = 411;
        }
        this.h = bounds.height;
        if (this.h < 231){
            this.h = 231;
        }
        localStorage.setItem(`${this.handle}-w`, this.w.toFixed(0).toString());
        localStorage.setItem(`${this.handle}-h`, this.h.toFixed(0).toString());
        if (this.model.size !== "maximized"){
            localStorage.setItem(`${this.handle}-x`, this.x.toFixed(0).toString());
            localStorage.setItem(`${this.handle}-y`, this.y.toFixed(0).toString());
        }
    }

    private move:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.moving){
            let x;
            let y;
            if (e instanceof TouchEvent){
                x = e.touches[0].clientX;
                y = e.touches[0].clientY;
            } else {
                x = e.clientX;
                y = e.clientY;
            }
            const bounds = this.getBoundingClientRect();
            let diffX = bounds.x - x - this.localX;
            let diffY = bounds.y - y - this.localY;
            this.x -= diffX;
            this.y -= diffY;
            this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        }
    }

    private stopMove:EventListener = (e:MouseEvent|TouchEvent) => {
        if (this.moving){
            this.save();
        }
        this.moving = false;
    }

    private startMove:EventListener = (e:MouseEvent|TouchEvent) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (this.model.size === "maximized"){
            return;
        }
        this.moving = true;
        let x;
        let y;
        if (e instanceof TouchEvent){
            x = e.touches[0].clientX;
            y = e.touches[0].clientY;
        } else {
            x = e.clientX;
            y = e.clientY;
        }
        const bounds = this.getBoundingClientRect();
        this.localX = bounds.x - x;
        this.localY = bounds.y - y;
    }

    private handleMinimize:EventListener = (e:Event) => {
        this.minimize();
    }

    private handleMaximize:EventListener = (e:Event) => {
        switch(this.model.size){
            case "normal":
                this.maximize();
                break;
            default:
                this.windowed();
                break;
        }
    }

    private handleClose:EventListener = (e:Event) => {
        this.close();
    }

    private noop:EventListener = (e:Event) => {
        e.preventDefault();
        e.stopImmediatePropagation();
    }

    private renderMaximizeIcon():TemplateResult {
        switch(this.model.size){
            case "normal":
                return html`
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-square" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <rect x="4" y="4" width="16" height="16" rx="2"></rect>
                    </svg>
                `;
            default:
                return html`
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-copy" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <rect x="8" y="8" width="12" height="12" rx="2"></rect>
                        <path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2"></path>
                    </svg>
                `;
        }
    }

    private renderContent():TemplateResult|string{
        let out;
        if (this.model.size !== "minimized"){
            out = html`
                <div class="container">
                    ${this.view}
                </div>
            `;
        } else {
            out = "";
        }
        return out;
    }

    override render(): void {
        this.style.width = `${this.w}px`;
        this.style.height = `${this.h}px`;
        this.setAttribute("window", this.name.toLowerCase().trim().replace(/\s+/g, "-"));
        this.setAttribute("size", this.model.size);
        const view = html`
            <div class="header" flex="row nowrap items-center justify-between" @mousedown=${this.startMove}>
                <h3 class="font-sm px-0.5">${this.name}</h3>
                <div class="h-full" flex="row nowrap items-center">
                    <button @click=${this.handleMinimize} @mousedown=${this.noop} sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-minus" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <line x1="5" y1="12" x2="19" y2="12"></line>
                        </svg>
                    </button>
                    <button @click=${this.handleMaximize} @mousedown=${this.noop} sfx="button">
                        ${this.renderMaximizeIcon()}
                    </button>
                    <button @click=${this.handleClose} @mousedown=${this.noop} sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <line x1="18" y1="6" x2="6" y2="18"></line>
                            <line x1="6" y1="6" x2="18" y2="18"></line>
                        </svg>
                    </button>
                </div>
            </div>
            ${this.renderContent()}
            ${new ResizeHandle(this, "x")}
            ${new ResizeHandle(this, "y")}
            ${new ResizeHandle(this, "both")}
        `;
        render(view, this);
    }
}
env.bind("window-component", Window);