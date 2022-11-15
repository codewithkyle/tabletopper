import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import Input from "~brixi/components/inputs/input/input";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import env from "~brixi/controllers/env";
import {send} from "~controllers/ws";

interface INPCModal {
    name: string,
    hp: number,
    ac: number,
    x: number,
    y: number,
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
                            callback: (value:number) => {
                                this.set({
                                    hp: value
                                }, true);
                            }
                        })}
                        ${new NumberInput({
                            name: "ac",
                            label: "Armour Class",
                            callback: (value:number) => {
                                this.set({
                                    ac: value,
                                }, true);
                            }
                        })}
                    </div>
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
