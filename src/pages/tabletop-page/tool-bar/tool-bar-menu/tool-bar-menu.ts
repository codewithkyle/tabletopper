import { publish } from "@codewithkyle/pubsub";
import { navigateTo } from "@codewithkyle/router";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import TabletopImageModal from "~components/tabletop-image-modal/tabletop-image-modal";
import { close } from "~controllers/ws";
import { ToolbarMenu as Menu} from "~types/app";

interface IToolbarMenu {
    menu: Menu,
};
export default class ToolbarMenu extends SuperComponent<IToolbarMenu>{
    constructor(menu:Menu){
        super();
        this.model = {
            menu: menu,
        }
    }

    override async connected(){
        env.css(["tool-bar-menu"]);
        this.render();
    }

    public changeMenu(menu:Menu):void{
        this.set({
            menu: menu,
        });
    }

    private calcOffsetX():number{
        const el = document.body.querySelector(`tool-bar button[data-menu="${this.model.menu}"]`);
        const bounds = el.getBoundingClientRect();
        return bounds.x;
    }

    private exit(){
        sessionStorage.clear();
        close();
        location.href = location.origin;
    }

    private close(){
        publish("toolbar", {
            type: "menu:close",
        });
        this.remove();
    }

    private loadImage(){
        const modal = new TabletopImageModal();
        document.body.append(modal);
        this.close();
    }

    private clickExit:EventListener = (e:Event) => {
        this.exit();
    }

    private clickLoadImage:EventListener = (e:Event) => {
        this.loadImage();
    }

    private renderFileMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button">
                    <span>Save</span>
                    <span>Ctrl+S</span>
                </button>
                <button sfx="button">
                    <span>Save As</span>
                    <span>Ctrl+Shift+S</span>
                </button>
                <button sfx="button">
                    <span>Load</span>
                    <span>Ctrl+L</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>Options</span>
                    <span>Ctrl+,</span>
                </button>
                <hr>
                <button sfx="button" @click=${this.clickExit}>
                    <span>Exit</span>
                    <span>Alt+F4</span>
                </button>
            </div> 
        `;
    }

    private renderTabletopMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button" @click=${this.clickLoadImage}>
                    <span>Load image</span>
                    <span>Ctrl+N</span>
                </button>
                <button sfx="button">
                    <span>Spawn pawns</span>
                    <span>Ctrl+L</span>
                </button>
                <button sfx="button">
                    <span>Clear tabletop</span>
                    <span>Ctrl+Backspace</span>
                </button>
            </div> 
        `;
    }

    private renderViewMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button">
                    <span>Initiative tracker</span>
                    <span>Ctrl+1</span>
                </button>
                <button sfx="button">
                    <span>Chat</span>
                    <span>Ctrl+2</span>
                </button>
                <button sfx="button">
                    <span>Monster manual</span>
                    <span>Ctrl+3</span>
                </button>
                <button sfx="button">
                    <span>Dice tray</span>
                    <span>Ctrl+4</span>
                </button>
                <button sfx="button">
                    <span>Drawing tools</span>
                    <span>Ctrl+5</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>Zoom in</span>
                    <span>Ctrl+=</span>
                </button>
                <button sfx="button">
                    <span>Zoom out</span>
                    <span>Ctrl+-</span>
                </button>
                <button sfx="button">
                    <span>Reset zoom</span>
                    <span>Ctrl+0</span>
                </button>
                <button sfx="button">
                    <span>Toggle fullscreen</span>
                    <span>F11</span>
                </button>
            </div> 
        `;
    }

    private renderHelpMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button">
                    <span>Report issue</span>
                </button>
                <button sfx="button">
                    <span>Show user guides</span>
                </button>
                <button sfx="button">
                    <span>Show keyboard shortcuts</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>Privacy policy</span>
                </button>
                <button sfx="button">
                    <span>License</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>About</span>
                </button>
            </div> 
        `;
    }

    private renderInitiativeMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button">
                    <span>Next</span>
                    <span>Ctrl+Shift+=</span>
                </button>
                <button sfx="button">
                    <span>Previous</span>
                    <span>Ctrl+Shift+-</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>Clear tracker</span>
                    <span>Ctrl+Shift+Backspace</span>
                </button>
                <button sfx="button">
                    <span>Sync tracker</span>
                    <span>Ctrl+R</span>
                </button>
            </div> 
        `;
    }

    private clickBackdrop:EventListener = (e:Event) => {
        this.close();
    }

    override render(): void {
        let menu;
        switch(this.model.menu){
            case "file":
                menu = this.renderFileMenu();
                break;
            case "tabletop":
                menu = this.renderTabletopMenu();
                break;
            case "initiative":
                menu = this.renderInitiativeMenu();
                break;
            case "help":
                menu = this.renderHelpMenu();
                break;
            case "view":
                menu = this.renderViewMenu();
                break;
            default:
                break;
        }
        const view = html`
            <div @click=${this.clickBackdrop} class="backdrop"></div> 
            ${menu}
        `;
        render(view, this);
    }
}
env.bind("tool-bar-menu", ToolbarMenu);