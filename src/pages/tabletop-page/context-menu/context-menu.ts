import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import env from "~brixi/controllers/env";
import SpotlightSearch from "~components/spotlight-search/spotlight-search";
import MonsterEditor from "~components/window/windows/monster-editor/monster-editor";
import { send } from "controllers/ws";
import db from "@codewithkyle/jsql";
import NPCModal from "./npc-modal/npc-modal";

interface IContextMenu{}
export default class ContextMenu extends SuperComponent<IContextMenu>{
    private x:number;
    private y:number;

    constructor(x:number, y:number){
        super();
        this.x = x;
        this.y = y;
    }

    public setLocation(x:number, y:number){
        this.x = x;
        this.y = y;
        this.render();
    }

    override async connected() {
        await env.css(["context-menu"]);
        this.render();
        document.body.addEventListener("click", ()=>{
            this.remove();
        }, { passive: true });
    }

    override render(){
        this.style.transform = `translate(${this.x}px, ${this.y}px)`;
        const view = html`
            ${new Button({
                label: "Spawn monster",
                kind: "text",
                color: "grey",
                class: "w-full",
                css: "justify-content: flex-start;",
                callback: ()=>{
                    const el = new SpotlightSearch(true, false, async (type:"monster"|"spell", index:string)=>{
                        if (index === null){
                            const windowEl = new Window({
                                name: "Create Monster",
                                view: new MonsterEditor(),
                                width: 600,
                                height: 350,
                            });
                        } else {
                            const monster = (await db.query<Monster>("SELECT * FROM monsters WHERE index = $index", { index: index }))[0];
                            send("room:tabletop:spawn:monster", {
                                x: 0,
                                y: 0,
                                index: index,
                                name: monster.name,
                            });
                        }
                    });
                    document.body.appendChild(el);
                    this.remove();
                }
            })}
            ${new Button({
                label: "Spawn NPC",
                kind: "text",
                color: "grey",
                class: "w-full",
                css: "justify-content: flex-start;",
                callback: ()=>{
                    const modal = new NPCModal();
                    document.body.appendChild(modal);
                    this.remove();
                }
            })}
        `;
        render(view, this);
    }
}
env.bind("context-menu", ContextMenu);
