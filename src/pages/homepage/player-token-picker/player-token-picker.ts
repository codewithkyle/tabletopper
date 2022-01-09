import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import PlayerTokenPickerModal from "./player-token-modal/player-token-modal";

interface IPlayerTokenPicker {
    selectedImageId: string;
}
export default class PlayerTokenPicker extends SuperComponent<IPlayerTokenPicker>{
    constructor(){
        super();
        this.model = {
            selectedImageId: null,
        };
    }

    override async connected(){
        await env.css(["player-token-picker", "player-token-picker-modal"]);
        this.render();
    }

    private imageUploadCallback(uid:string|null){
        if (uid !== null){
            this.set({
                selectedImageId: uid,
            });
        }
    }

    public async getValue():Promise<string>{
        const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: this.model.selectedImageId, }))[0];
        return image.data;
    }

    private openModal:EventListener = (e:Event) => {
        const modal = new PlayerTokenPickerModal(this.imageUploadCallback.bind(this));
        document.body.appendChild(modal);
    }

    private async renderImage(){
        const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: this.model.selectedImageId, }))[0];
        return html`
            <button @click=${this.openModal} class="picker" sfx="button" tooltip="Change token">
                <img src="${image.data}" alt="${image.name}" draggable="false">
            </button>
        `;
    }

    private renderEmpty(){
        return html`
            <button @click=${this.openModal} class="picker" sfx="button" tooltip="Select token">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-photo" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                    <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                    <line x1="15" y1="8" x2="15.01" y2="8"></line>
                    <rect x="4" y="4" width="16" height="16" rx="3"></rect>
                    <path d="M4 15l4 -4a3 5 0 0 1 3 0l5 5"></path>
                    <path d="M14 14l1 -1a3 5 0 0 1 3 0l2 2"></path>
                </svg>
            </button>
        `;
    }

    override async render() {
        const view = html`
            ${this.model.selectedImageId ? await this.renderImage() : this.renderEmpty()}
        `;
        render(view, this);
    }
}
env.bind("player-token-picker", PlayerTokenPicker);