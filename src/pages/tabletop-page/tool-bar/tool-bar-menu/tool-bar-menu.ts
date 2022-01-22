import { publish, subscribe } from "@codewithkyle/pubsub";
import { navigateTo } from "@codewithkyle/router";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import TabletopImageModal from "~components/tabletop-image-modal/tabletop-image-modal";
import Window from "~components/window/window";
import PlayerMenu from "~components/window/windows/player-menu/player-menu";
import { close, send } from "~controllers/ws";
import type { ToolbarMenu as Menu} from "~types/app";

interface IToolbarMenu {
    menu: Menu,
};
export default class ToolbarMenu extends SuperComponent<IToolbarMenu>{
    private zoom: number;

    constructor(menu:Menu){
        super();
        this.zoom = sessionStorage.getItem("zoom") != null ? parseFloat(sessionStorage.getItem("zoom")) : 1;
        this.model = {
            menu: menu,
        };
        subscribe("tabletop", this.tabletopInbox.bind(this));
    }

    private tabletopInbox({ type, data }){
        switch(type){
            case "zoom":
                this.zoom = data;
                break;
            default:
                break;
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
        sessionStorage.removeItem("room");
        sessionStorage.removeItem("lastSocketId");
        send("room:quit");
        this.close();
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

    private clearImage:EventListener = (e:Event) => {
        send("room:tabletop:map:clear");
        this.close();
    }

    private clickExit:EventListener = (e:Event) => {
        this.exit();
    }

    private clickLoadImage:EventListener = (e:Event) => {
        this.loadImage();
    }

    private toggleFullscreen:EventListener = (e:Event) => {
        if (window.innerHeight === screen.height){
            document.exitFullscreen();
        } else {
            document.documentElement.requestFullscreen();
        }
        this.close();
    }

    private zoomReset:EventListener = (e:Event) => {
        let zoom = 1;
        publish("tabletop", {
            type: "zoom",
            data: zoom,
        });
        this.close();
    }

    public zoom200:EventListener = (e:Event) => {
        let zoom = 2;
        publish("tabletop", {
            type: "zoom",
            data: zoom,
        });
        this.close();
    }

    private zoomOut:EventListener = (e:Event) => {
        let zoom  = this.zoom - 0.1;
        if (zoom < 0.1){
            zoom = 0.1;
        }
        publish("tabletop", {
            type: "zoom",
            data: zoom,
        });
    }

    private zoomIn:EventListener = (e:Event) => {
        let zoom = this.zoom + 0.1;
        if (zoom > 2){
            zoom = 2;
        }
        publish("tabletop", {
            type: "zoom",
            data: zoom,
        });
    }
    
    private centerTabletop:EventListener = (e:Event) => {
        publish("tabletop", {
            type: "position:reset",
        });
        this.close();
    }

    private spawnPawns:EventListener = async (e:Event) => {
        send("room:tabletop:spawn:players");
        this.close();
    }

    private copyRoomCode:EventListener = async (e:Event) => {
        await navigator.clipboard.writeText(sessionStorage.getItem("room"));
        notifications.snackbar("Room code copied to clipboard.");
        this.close();
    }

    private lockRoom:EventListener = (e:Event) => {
        send("room:lock");
        this.close();
    }

    private unlockRoom:EventListener = (e:Event) => {
        send("room:unlock");
        this.close();
    }

    private openPlayerMenu:EventListener = (e:Event) => {
        const window = document.body.querySelector('window-component[window="players"]') || new Window("Players", new PlayerMenu());
        if (!window.isConnected){
            document.body.append(window);
        }
        this.close();
    }

    private renderFileMenu():TemplateResult{
        if (sessionStorage.getItem("role") === "gm"){
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
                        <span>Export Spells</span>
                    </button>
                    <button sfx="button">
                        <span>Export Monsters</span>
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
        else {
            return html`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Export Spells</span>
                    </button>
                    <button sfx="button">
                        <span>Export Monsters</span>
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
    }

    private renderTabletopMenu():TemplateResult|string{
        if (sessionStorage.getItem("role") === "gm"){
            return html`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button" @click=${this.clickLoadImage}>
                        <span>Load image</span>
                        <span>Ctrl+N</span>
                    </button>
                    <button sfx="button" @click=${this.spawnPawns}>
                        <span>Spawn pawns</span>
                        <span>Ctrl+L</span>
                    </button>
                    <button sfx="button" @click=${this.clearImage}>
                        <span>Clear tabletop</span>
                        <span>Ctrl+Backspace</span>
                    </button>
                </div> 
            `;
        } else {
            return "";
        }
    }

    private renderWindowsMenu():TemplateResult{
        if (sessionStorage.getItem("role") === "gm"){
            return html`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Initiative tracker</span>
                    </button>
                    <button sfx="button">
                        <span>Chat</span>
                    </button>
                    <button sfx="button">
                        <span>Monster manual</span>
                    </button>
                    <button sfx="button">
                        <span>Spellbook</span>
                    </button>
                    <button sfx="button">
                        <span>Dice tray</span>
                    </button>
                    <button sfx="button">
                        <span>Drawing tools</span>
                    </button>
                    <button sfx="button">
                        <span>Music</span>
                    </button>
                </div>
            `;
        }
        else {
            return html`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Initiative tracker</span>
                    </button>
                    <button sfx="button">
                        <span>Chat</span>
                    </button>
                    <button sfx="button">
                        <span>Spellbook</span>
                    </button>
                    <button sfx="button">
                        <span>Dice tray</span>
                    </button>
                    <button sfx="button">
                        <span>Drawing tools</span>
                    </button>
                </div>
            `;
        }
    }

    private renderViewMenu():TemplateResult{
        return html`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button" @click=${this.zoomIn}>
                    <span>Zoom in</span>
                    <span>Ctrl+=</span>
                </button>
                <button sfx="button" @click=${this.zoomOut}>
                    <span>Zoom out</span>
                    <span>Ctrl+-</span>
                </button>
                <button sfx="button" @click=${this.zoomReset}>
                    <span>100%</span>
                    <span>Ctrl+0</span>
                </button>
                <button sfx="button" @click=${this.zoom200}>
                    <span>200%</span>
                    <span>Ctrl+0</span>
                </button>
                <hr>
                <button sfx="button" @click=${this.toggleFullscreen}>
                    <span>Toggle fullscreen</span>
                    <span>F11</span>
                </button>
                <button sfx="button" @click=${this.centerTabletop}>
                    <span>Center tabletop</span>
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

    private renderInitiativeMenu():TemplateResult|string{
        if (sessionStorage.getItem("role") === "gm"){
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
        else {
            return "";
        }
    }

    private renderRoomMenu():TemplateResult|string{
        if (sessionStorage.getItem("role") === "gm"){
            return html`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button" @click=${this.lockRoom}>
                        <span>Lock room</span>
                    </button>
                    <button sfx="button" @click=${this.unlockRoom}>
                        <span>Unlock room</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.copyRoomCode}>
                        <span>Copy room code</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.openPlayerMenu}>
                        <span>Player list</span>
                    </button>
                </div> 
            `;
        }
        else {
            return "";
        }
    }

    private clickBackdrop:EventListener = (e:Event) => {
        this.close();
    }

    override render(): void {
        let menu;
        switch(this.model.menu){
            case "room":
                menu = this.renderRoomMenu();
                break;
            case "window":
                menu = this.renderWindowsMenu();
                break;
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