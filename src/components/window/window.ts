import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import ResizeHandle from "~components/resize-handle/resize-handle";

interface IWindow {
    size: "minimized" | "maximized" | "normal";
};
export interface Settings {
    name:string,
    view:HTMLElement | TemplateResult | string,
    handle?: string,
    width?: number,
    height?: number,
    minWidth?: number,
    minHeight?: number,
}
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
    private minWidth: number;
    private minHeight: number;

    constructor(settings:Settings){
        super();
        this.handle = settings?.handle ?? settings.name.toLowerCase().trim().replace(/\s+/g, "-");

        this.minWidth = settings?.minWidth ?? 411;
        this.minHeight = settings?.minHeight ?? 231;

        const savedX = localStorage.getItem(`${this.handle}-x`);
        const savedY = localStorage.getItem(`${this.handle}-y`);
        this.x = savedX ? parseInt(savedX) : 0;
        this.y = savedY ? parseInt(savedY) : 28;
        if (savedX == null){
            localStorage.setItem(`${this.handle}-x`, this.x.toFixed(0).toString());
        }
        if (savedY == null){
            localStorage.setItem(`${this.handle}-y`, this.y.toFixed(0).toString());
        }

        const savedWidth = localStorage.getItem(`${this.handle}-w`);
        const savedHeight = localStorage.getItem(`${this.handle}-h`);
        if (savedWidth != null && savedHeight != null){
            this.w = parseInt(savedWidth);
            this.h = parseInt(savedHeight);
        } else {
            this.w = settings?.width ?? this.minWidth;
            this.h = settings?.height ?? this.minHeight;
        }
        if (savedWidth == null){
            localStorage.setItem(`${this.handle}-w`, this.w.toFixed(0).toString());
        }
        if (savedHeight == null){
            localStorage.setItem(`${this.handle}-h`, this.h.toFixed(0).toString());
        }

        this.resize();

        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        this.view = settings.view;
        this.name = settings.name;
        this.moving = false;
        this.model = {
            size: "normal",
        };
    }

    override async connected(){
        await env.css(["window"]);
        window.addEventListener("mouseup", this.stopMove, { capture: true, passive: true });
        window.addEventListener("mousemove", this.move, { capture: true, passive: true });
        window.addEventListener("touchend", this.stopMove, { capture: true, passive: true });
        window.addEventListener("touchmove", this.move, { capture: true, passive: true });
        window.addEventListener("resize", this.debounce(this.resizeEvent, 300), { capture: true, passive: true });
        this.render();
        this.focus();
    }

    private resize(){
        if (this.x >= window.innerWidth){
            this.x = window.innerWidth - this.w;
        }
        if (this.y >= window.innerHeight){
            this.y = window.innerHeight - this.h;
        }
    }

    private resizeEvent:EventListener = (e) => {
        this.resize();
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
    }

    public focus():void{
        document.body.querySelectorAll("window-component").forEach((window:Window) => {
            window.blur();
        });
        this.style.zIndex = "1001";
    }

    public blur(): void {
       this.style.zIndex = "1000";
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
        this.focus();
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
        this.focus();
    }
    
    public close(){
        this.remove();
    }

    public save(){
        const bounds = this.getBoundingClientRect();
        this.x = bounds.x;
        this.y = bounds.y;
        this.w = bounds.width;
        if (this.w < this.minWidth){
            this.w = this.minWidth;
        }
        this.h = bounds.height;
        if (this.h < this.minHeight){
            this.h = this.minHeight;
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
            if (window.TouchEvent && e instanceof TouchEvent){
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
        //e.preventDefault();
        //e.stopImmediatePropagation();
        if (this.model.size === "maximized"){
            return;
        }
        this.moving = true;
        let x;
        let y;
        if (window.TouchEvent && e instanceof TouchEvent){
            x = e.touches[0].clientX;
            y = e.touches[0].clientY;
        } else {
            x = e.clientX;
            y = e.clientY;
        }
        const bounds = this.getBoundingClientRect();
        this.localX = bounds.x - x;
        this.localY = bounds.y - y;
        this.focus();
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
            <div class="header" flex="row nowrap items-center justify-between" @mousedown=${this.startMove} @touchstart=${this.startMove}>
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
            ${new ResizeHandle(this, "x", this.minWidth, this.minHeight)}
            ${new ResizeHandle(this, "y", this.minWidth, this.minHeight)}
            ${new ResizeHandle(this, "both", this.minWidth, this.minHeight)}
        `;
        render(view, this);
    }
}
env.bind("window-component", Window);
