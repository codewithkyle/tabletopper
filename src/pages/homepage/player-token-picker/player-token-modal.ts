import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import { UUID } from "~brixi/types/uuid";
import { Base64EncodeFile } from "~utils/file";

interface IPlayerTokenPickerModal {
    callback: Function,
}
export default class PlayerTokenPickerModal extends SuperComponent<IPlayerTokenPickerModal>{
    constructor(callback:Function){
        super();
        this.model = {
            callback: callback,
        };
        this.render();
    }

    private close:EventListener = (e:Event) => {
        this.model.callback(null);
        this.remove();
    }

    private handleInput:EventListener = async (e:Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const files = input.files;
        if (files.length){
            const image = files[0];
            const data = await Base64EncodeFile(image);
            const uid = UUID();
            await db.query("INSERT INTO images VALUES ($img)", {
                img: {
                    uid: uid,
                    name: image.name,
                    data: data,
                    type: "player-token",
                },
            });
            this.model.callback(uid);
            this.remove();
        }
    }

    private selectImage:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        this.model.callback(target.dataset.uid);
        this.remove();
    }

    override async render() {
        const images = await db.query("SELECT * FROM images WHERE type = 'player-token'");
        const view = html`
           <div class="backdrop" @click=${this.close}></div>
           <div class="modal">
               <button class="close" @click=${this.close}>
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                    </svg>
               </button>
               <h2 class="block font-lg pb-0.5 mb-1 font-medium font-grey-900 line-normal border-b-1 border-b-solid border-b-grey-200">Your Tokens</h2>
               <div class="content">
                   <div class="upload-image-button">
                       <label for="token-upload">
                            <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-upload" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"></path>
                                <polyline points="7 9 12 4 17 9"></polyline>
                                <line x1="12" y1="4" x2="12" y2="16"></line>
                            </svg>
                            <span>Upload</span>
                       </label>
                       <input @change=${this.handleInput} type="file" accept="image/png, image/jpg, image/jpeg" id="token-upload" name="token">
                    </div>
                   ${images.map(img => {
                       return html`
                            <button class="token-button" data-uid="${img.uid}" @click=${this.selectImage}>
                                <img src="${img.data}" alt="${img.name}" draggalbe="false">
                            </button>
                        `;
                   })}
               </div>
           </div>
        `;
        render(view, this);
    }
}
env.bind("player-token-picker-modal", PlayerTokenPickerModal);