import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import { ConvertBase64ToBlob } from "~utils/file";

export interface IPlayerPawn {
    name: string,
    uid: string,
    token: string,
    image: string,
    x: number,
    y: number,
};
export default class PlayerPawn extends SuperComponent<IPlayerPawn>{
    constructor(player){
        super();
        this.model = {
            name: player.name,
            uid: player.uid,
            token: player.token,
            image: null,
            x: player.x,
            y: player.y,
        };
    }
    
    override async connected(){
        await env.css(["player-pawn"]);
        if (this.model.token){
            const image = (await db.query("SELECT * FROM images WHERE uid = $uid", { uid: this.model.token }))[0];
            const blob = ConvertBase64ToBlob(image.data);
            const url = URL.createObjectURL(blob);
            this.set({
                image: url,
            });
        }
        this.addEventListener("mousemove", this.noopEvent, { passive: false, capture: true });
        this.addEventListener("mousedown", this.noopEvent, { passive: false, capture: true });
        this.render();
    }

    private noopEvent:EventListener = (e:Event) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        return false;
    }

    private renderPawn():TemplateResult{
        let out;
        if (this.model.image){
            out = html`
                <img src="${this.model.image}" alt="${this.model.name} token" draggable="false">
            `;
        } else {
            out = html`
                <div class="generic-pawn"></div>
            `;
        }
        return out;
    }

    override async render() {
        this.dataset.uid = this.model.uid;
        this.setAttribute("tooltip", this.model.name);
        this.setAttribute("sfx", "button");
        this.style.transform = `transform(${this.model.x}px, ${this.model.y}px)`;
        const view = html`
            ${this.renderPawn()}
        `;
        render(view, this);
    }
}
env.bind("player-pawn", PlayerPawn);