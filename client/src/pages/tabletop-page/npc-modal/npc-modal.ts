import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import "~brixi/components/inputs/input/input";
import "~brixi/components/inputs/number-input/number-input";
import "~brixi/components/select/select";
import env from "~brixi/controllers/env";
import { send } from "~controllers/ws";
import { Size } from "~types/app";
import TabeltopComponent from "../tabletop-component/tabletop-component";

interface INPCModal {
    name: string,
    hp: number,
    ac: number,
    x: number,
    y: number,
    size: Size,
}
export default class NPCModal extends SuperComponent<INPCModal>{
    constructor(x:number, y:number){
        super();
        this.model = {
            name: "",
            hp: 0,
            ac: 0,
            x: x,
            y: y,
            size: "medium",
        };
    }

    override async connected(){
        await env.css(["npc-modal"]);
        this.render();
    }

    private clickClose = () => {
        this.remove();
    }

    override render(){
        const view = html`
            <div class="backdrop" @click=${this.clickClose}></div>
            <div class="modal">
                <div class="w-full p-1.5" grid="rows 1 gap-1.5">
                    <input-component
                        data-name="name"
                        data-label="Name"
                    ></input-component>
                    <div class="w-full" grid="columns 2 gap-1.5">
                        <number-input-component
                            data-name="hp"
                            data-label="Hit Points"
                            data-icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19.5 12.572l-7.5 7.428l-7.5 -7.428m0 0a5 5 0 1 1 7.5 -6.566a5 5 0 1 1 7.5 6.572"></path></svg>'
                        ></number-input-component>
                        <number-input-component
                            data-name="ac"
                            data-label="Armour Class"
                            data-icon='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"></path></svg>'
                        ></number-input-component>
                    </div>
                    <select-component
                        data-name="size"
                        data-label="Size"
                        data-value="medium"
                        data-options='[{ label: "Tiny", value: "tiny", },{ label: "Small", value: "small", },{ label: "Medium", value: "medium", },{ label: "Large", value: "large", },{ label: "Huge", value: "huge", },{ label: "Gargantuan", value: "gargantuan", },]'
                    ></select-component>
                </div>
                <div class="w-full border-t-1 border-t-solid border-t-grey-300 bg-grey-50 p-1" flex="items-center justify-end">
                    <button
                        class="bttn mr-1"
                        kind="solid"
                        color="white"
                    >Cancel</button>
                    <button
                        class="bttn"
                        kind="solid"
                        color="success"
                        @click=${()=>{
                            const npc = this.get();
                            const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
                            let diffX = (this.model.x - tabletop.x) / tabletop.zoom;
                            let diffY = (this.model.y - tabletop.y) / tabletop.zoom;
                            npc.x = Math.round(diffX) - 16;
                            npc.y = Math.round(diffY) - 16;
                            send("room:tabletop:spawn:npc", npc);
                            this.remove();
                        }}
                    >Create</button>
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("npc-modal", NPCModal);
