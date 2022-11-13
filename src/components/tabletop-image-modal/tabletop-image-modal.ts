import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { UUID } from "@codewithkyle/uuid";
import { html, render, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import cc from "~controllers/control-center";
import { send } from "~controllers/ws";
import { Base64EncodeFile } from "~utils/file";

interface ITabletopImageModal {
    selected: string,
    images: Array<any>,
}
export default class TabletopImageModal extends SuperComponent<ITabletopImageModal>{
    constructor(){
        super();
        this.model = {
            selected: null,
            images: [],
        };
        this.state = "LOADING";
        this.stateMachine = {
            LOADING: {
                READY: "IDLING",
                LOAD:" LOADING",
            },
            IDLING: {
                LOAD: "LOADING",
            },
        };
    }

    override async connected(){
        await env.css(["tabletop-image-modal"]);
        this.render();
        const images = await db.query("SELECT * FROM images WHERE type = map");
        this.set({
            images: images,
        });
        this.trigger("READY");
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

    private clickClose:EventListener = (e:Event) => {
        this.close();
    }

    private handleInput:EventListener = async (e:Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const files = input.files;
        if (files.length){
            const image = files[0];
            if (image.size < 10_000_000){ // Smaller than 10MB
                this.trigger("LOAD");
                const data = await Base64EncodeFile(image);
                const uid = UUID();
                this.set({
                    selected: uid,
                }, true);
                await db.query("INSERT INTO images VALUES ($img)", {
                    img: {
                        uid: uid,
                        name: image.name,
                        data: data,
                        type: "map",
                    },
                });
                this.load();
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
                ${this.model.images.map(img => {
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

    override async render() {
        let content;
        switch(this.state){
            case "LOADING":
                content = html`
                    ${new Spinner({
                        color: "grey",
                        css: "z-index: 5;",
                        size: 32,
                    })}
                `;
                break;
            default:
                content = html`
                    <div class="modal">
                        <h2 class="font-grey-800 font-bold block px-1.5 pt-1.25 pb-1.25 border-b-solid border-b-1 border-b-grey-300">Tabletop Images</h2>
                        ${new Button({
                            callback: this.close.bind(this),
                            kind: "text",
                            color: "grey",
                            icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>`,
                            class: "close",
                            iconPosition: "center",
                            shape: "round",
                        })}
                        ${await this.renderImages()}
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
                break;
        }
        const view = html`
            <div class="backdrop" @click=${this.clickClose}></div>
            ${content}
        `;
        render(view, this);
    }
}
env.bind("tabletop-image-modal", TabletopImageModal);
