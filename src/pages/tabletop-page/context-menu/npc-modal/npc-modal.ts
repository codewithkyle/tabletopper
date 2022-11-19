import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import Input from "~brixi/components/inputs/input/input";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import Select from "~brixi/components/select/select";
import env from "~brixi/controllers/env";
import {send} from "~controllers/ws";
import {Size} from "~types/app";

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
                    ${new Input({
                        name: "name",
                        label: "Name",
                        callback: (value:string) => {
                            this.set({
                                name: value,
                            }, true);
                        }
                    })}
                    <div class="w-full" grid="columns 2 gap-1.5">
                        ${new NumberInput({
                            name: "hp",
                            label: "Hit Points",
                            icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M19.5 12.572l-7.5 7.428l-7.5 -7.428m0 0a5 5 0 1 1 7.5 -6.566a5 5 0 1 1 7.5 6.572"></path></svg>`,
                            callback: (value:number) => {
                                this.set({
                                    hp: value
                                }, true);
                            }
                        })}
                        ${new NumberInput({
                            name: "ac",
                            label: "Armour Class",
                            icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"></path></svg>`,
                            callback: (value:number) => {
                                this.set({
                                    ac: value,
                                }, true);
                            }
                        })}
                    </div>
                    ${new Select({
                        name: "size",
                        label: "Size",
                        value: "medium",
                        options: [
                            { label: "Tiny", value: "tiny", },
                            { label: "Small", value: "small", },
                            { label: "Medium", value: "medium", },
                            { label: "Large", value: "large", },
                            { label: "Huge", value: "huge", },
                            { label: "Gargantuan", value: "gargantuan", },
                        ],
                        callback: (value:string) => {
                            this.set({
                                size: value,
                            }, true);
                        }
                    })}
                </div>
                <div class="w-full border-t-1 border-t-solid border-t-grey-300 bg-grey-50 p-1" flex="items-center justify-end">
                    ${new Button({
                        kind: "solid",
                        color: "white",
                        label: "Cancel",
                        class: "mr-1",
                        callback: ()=>{
                            this.remove();
                        }
                    })}
                    ${new Button({
                        kind: "solid",
                        color: "success",
                        label: "Create",
                        callback: ()=>{
                            const npc = this.get();
                            const tabletop = document.body.querySelector("tabletop-component");
                            let diffX = (this.model.x - tabletop.x) / tabletop.zoom;
                            let diffY = (this.model.y - tabletop.y) / tabletop.zoom;
                            npc.x = Math.round(diffX) - 16;
                            npc.y = Math.round(diffY) - 16;
                            send("room:tabletop:spawn:npc", npc);
                            this.remove();
                        }
                    })}
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("npc-modal", NPCModal);
