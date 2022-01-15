import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { UUID } from "@codewithkyle/uuid";
import { html, render, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import Lightswitch from "~brixi/components/lightswitch/lightswitch";
import Select, { SelectOption } from "~brixi/components/select/select";
import Tabs from "~brixi/components/tabs/tabs";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import cc from "~controllers/control-center";
import { send } from "~controllers/ws";
import { Base64EncodeFile } from "~utils/file";

interface ITabletopImageModal {
    tab: string,
    selected: string,
}
export default class TabletopImageModal extends SuperComponent<ITabletopImageModal>{
    constructor(){
        super();
        this.model = {
            tab: "images",
            selected: null,
        };
    }

    override async connected(){
        await env.css(["tabletop-image-modal"]);
        this.render();
    }

    private close(){
        this.remove();
    }

    private async load(){
        const prevMaps = (await db.query("SELECT loaded_maps FROM games WHERE room = $room", { room: sessionStorage.getItem("room") }))[0].loaded_maps;
        let alreadyLoadedMap = false;
        for (let i = 0; i < prevMaps.length; i++){
            if (prevMaps[i] === this.model.selected){
                alreadyLoadedMap = true;
                break;
            }
        }
        if (!alreadyLoadedMap){
            const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: this.model.selected }))[0];
            const op = cc.insert("images", image.uid, image);
            cc.dispatch(op);
        }
        send("room:tabletop:map:load", this.model.selected);
        this.remove();
    }

    private handleTabSwitch(tab:string):void{
        this.set({
            tab: tab,
        });
    }

    private clickClose:EventListener = (e:Event) => {
        this.close();
    }

    private handleInput:EventListener = async (e:Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const files = input.files;
        if (files.length){
            const image = files[0];
            if (image.size < 10000000){
                const data = await Base64EncodeFile(image);
                const uid = UUID();
                await db.query("INSERT INTO images VALUES ($img)", {
                    img: {
                        uid: uid,
                        name: image.name,
                        data: data,
                        type: "map",
                    },
                });
                this.set({
                    selected: uid,
                });
            } else {
                // TODO: figure out how to reduce image quality using WASM? Take a look at Squoosh.app for more details
                notifications.error("Upload Failed", "Files must be 10MB or smaller.");
            }
        }
    }

    private selectImage:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        const uid = target.dataset.uid;
        this.set({
            selected: uid,
        });
    }

    private async renderImages():Promise<TemplateResult>{
        const images = await db.query("SELECT * FROM images WHERE type = map");
        return html`
            <div class="images">
                <div class="upload-image-button" sfx="button">
                    <label for="upload">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-upload" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"></path>
                            <polyline points="7 9 12 4 17 9"></polyline>
                            <line x1="12" y1="4" x2="12" y2="16"></line>
                        </svg>
                        <span>Upload</span>
                    </label>
                    <input @change=${this.handleInput} type="file" accept="image/png, image/jpg, image/jpeg" id="upload" name="token">
                </div>
                ${images.map(img => {
                    return html`
                        <button sfx="button" class="token-button ${this.model.selected === img.uid ? "is-selected" : ""}" data-uid="${img.uid}" @click=${this.selectImage} aria-label="${img.name}" tooltip>
                            <img src="${img.data}" alt="${img.name}" draggalbe="false">
                            ${this.model.selected === img.uid ? html`
                                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-circle-check" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                    <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                    <circle cx="12" cy="12" r="9"></circle>
                                    <path d="M9 12l2 2l4 -4"></path>
                                </svg>
                            ` : ""}
                        </button>
                    `;
                })}
            </div>
        `;
    }

    private renderSettings():TemplateResult{
        return html`
            <div class="settings">
                ${new NumberInput({
                    name: "gridSize",
                    label: "Grid Size",
                    required: true,
                    value: localStorage.getItem("gridSize") ? parseInt(localStorage.getItem("gridSize")) : 32,
                    class: "mb-1.5",
                    callback: (value) => {
                        localStorage.setItem("gridSize", value);
                    }
                })}
                ${new Select({
                    name: "fog",
                    class: "mb-1.5",
                    label: "Fog of War",
                    value: localStorage.getItem("fogOfWar") || "no",
                    callback: (option:SelectOption) => {
                        localStorage.setItem("fogOfWar", option.value.toString());
                    },
                    options: [
                        {
                            label: "No Fog of War",
                            value: "no",
                        },
                        {
                            label: "Manual Fog of War",
                            value: "manual",
                        },
                        {
                            label: "Auto Fog of War",
                            value: "auto",
                        }
                    ],
                })}
                ${new Lightswitch({
                    name: "autoLines",
                    label: "No Lines",
                    altLabel: "Auto Lines",
                    enabled: localStorage.getItem("autoLines") ? true : false,
                    class: "mb-1.5",
                    callback: (enabled) => {
                        if (enabled){
                            localStorage.setItem("autoLines", "true");
                        } else {
                            localStorage.removeItem("autoLines");
                        }
                    },
                })}
            </div>
        `;
    }

    override async render() {
        let content;
        switch(this.model.tab){
            case "images":
                content = await this.renderImages();
                break;
            case "settings":
                content = this.renderSettings();
                break;
            default:
                break;
        }
        const view = html`
            <div class="backdrop" @click=${this.clickClose}></div>
            <div class="modal">
                <h2 class="font-grey-800 font-bold block px-1.5 pt-1.5 pb-1">Tabletop Images</h2>
                <div class="block w-full px-1 border-b-1 border-b-solid border-b-grey-200">
                    ${new Tabs({
                        active: this.model.tab,
                        tabs: [
                            {
                                label: "Images",
                                value: "images",
                                icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-photo" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="15" y1="8" x2="15.01" y2="8"></line><rect x="4" y="4" width="16" height="16" rx="3"></rect><path d="M4 15l4 -4a3 5 0 0 1 3 0l5 5"></path><path d="M14 14l1 -1a3 5 0 0 1 3 0l2 2"></path></svg>`,
                            },
                            {
                                label: "Settings",
                                value: "settings",
                                icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-settings" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37c1 .608 2.296 .07 2.572 -1.065z"></path><circle cx="12" cy="12" r="3"></circle></svg>`,
                            }
                        ],
                        callback: this.handleTabSwitch.bind(this)
                    })}
                </div>
                ${new Button({
                    callback: this.close.bind(this),
                    kind: "text",
                    color: "grey",
                    icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>`,
                    class: "close",
                    iconPosition: "center",
                    shape: "round",
                })}
                ${content}
                <div class="w-full border-t-solid border-t-1 border-t-grey-200 bg-grey-50 p-1" flex="row nowrap items-center justify-end">
                    ${new Button({
                        callback: this.close.bind(this),
                        kind: "solid",
                        color: "white",
                        label: "cancel",
                        class: "mr-1"
                    })}
                    ${new Button({
                        kind: "solid",
                        color: "success",
                        callback: this.load.bind(this),
                        label: "load image",
                        disabled: this.model.selected?.length ? false : true,
                    })}
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("tabletop-image-modal", TabletopImageModal);