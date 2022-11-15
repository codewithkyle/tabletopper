import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
import Pawn from "../pawn";

export default class MonsterPawn extends Pawn{

    constructor(pawn){
        super(pawn); 
    }
    
    override async connected(){
        await env.css(["monster-pawn"]);
        this.render();
    }

    private renderPawn():TemplateResult{
        let out:TemplateResult;
        if (this.model.image){
            out = html`
                <img src="${this.model.image}" alt="${this.model.name} token" draggable="false">
            `;
        } else {
            out = html`
                <div class="monster-pawn"></div>
            `;
        }
        return out;
    }

    override async render() {
        if (this.model.hidden){
            this.style.visibility = "hidden";
            this.style.opacity = "0";
            this.style.pointerEvents = "none";
        } else {
            this.style.visibility = "visible";
            this.style.opacity = "1";
            this.style.pointerEvents = "all";
        }
        this.dataset.uid = this.model.uid;
        if (!this.dragging){
            this.setAttribute("tooltip", this.model.name);
            this.setAttribute("sfx", "button");
        }
        this.style.transform = `translate(${this.model.x}px, ${this.model.y}px)`;
        this.localX = this.model.x;
        this.localY = this.model.y;
        this.className = "pawn";
        const view = html`
            ${this.renderPawn()}
        `;
        render(view, this);
    }
}
env.bind("monster-pawn", MonsterPawn);
