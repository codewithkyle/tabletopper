import { subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import { ToolbarMenu as Menu } from "~types/app";
import ToolbarMenu from "./tool-bar-menu/tool-bar-menu";

interface IToolbar {}

export default class Toolbar extends SuperComponent<IToolbar>{
    private menuOpen: boolean;

    constructor(){
        super();
        this.menuOpen = false;
        subscribe("toolbar", this.inbox.bind(this));
    }

    private inbox({type, data}){
        switch(type){
            case "menu:close":
                this.menuOpen = false;
                this.querySelectorAll("button[data-menu]").forEach(el => {
                    el.classList.remove("is-open");
                });
                break;
            default:
                break;
        }
    }

    override async connected(){
       await env.css(["tool-bar"]);
       this.render();
    }

    private handleClick:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLButtonElement;
        const menu = target.dataset.menu as Menu;
        this.querySelectorAll("button[data-menu]").forEach(el => {
            el.classList.remove("is-open");
        });
        target.classList.add("is-open");
        this.menuOpen = true;
        const menuEl:ToolbarMenu = document.body.querySelector("tool-bar-menu") || new ToolbarMenu(menu);
        if (!menuEl.isConnected){
            document.body.append(menuEl);
        } else {
            menuEl.changeMenu(menu);
        }
    }

    private handleMouseEnter:EventListener = (e:MouseEvent) => {
        if (this.menuOpen){
            const target = e.currentTarget as HTMLButtonElement;
            const menu = target.dataset.menu as Menu;
            this.querySelectorAll("button[data-menu]").forEach(el => {
                el.classList.remove("is-open");
            });
            target.classList.add("is-open");
            const menuEl:ToolbarMenu = document.body.querySelector("tool-bar-menu") || new ToolbarMenu(menu);
            if (!menuEl.isConnected){
                document.body.append(menuEl);
            } else {
                menuEl.changeMenu(menu);
            }
        }
    }

    override render(): void {
       const view = html`
            <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="file">File</button>
            <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="tabletop">Tabletop</button>
            <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="initiative">Initiative</button>
            <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="view">View</button>
            <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="help">Help</button>
       `;
       render(view, this); 
    }
}
env.bind("tool-bar", Toolbar);