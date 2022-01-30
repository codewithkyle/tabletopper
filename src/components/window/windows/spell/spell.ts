import db from "@codewithkyle/jsql";
import { publish } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import type { Spell as S } from "~types/app";

interface ISpell extends S{}
export default class Spell extends SuperComponent<ISpell>{

    constructor(index:string){
        super();
        this.model = {
            index: index,
            name: null,
            desc: null,
            range: null,
            components: null,
            ritual: null,
            duration: null,
            castingTime: null,
            level: null,
            attackType: null,
            damageType: null,
            damage: null,
            school: null,
            classes: null,
            subsclasses: null,
            material: null,
        };
    }

    override async connected() {
        await env.css(["spell"]);
        const spell = (await db.query("SELECT * FROM spells WHERE index = $index", {
            index: this.model.index,
        }))[0];
        this.set(spell);
    }

    private async deleteSpell(){
        await db.query("DELETE FROM spells WHERE index = $index", {
            index: this.model.index,
        });
        const window = this.closest("window-component");
        window.remove();
        notifications.snackbar(`${this.model.name} has been deleted.`);
        publish("spells", {
            type: "refresh",
        });
    }

    private editSpell(){

    }

    private renderDamageTable():TemplateResult{
        let out;
        if (this.model.damage !== null){
            out = html`
                <hr class="block w-full border-b-1 border-b-solid border-b-grey-200 my-1">
                <div class="block w-full p-1">
                    <table>
                        <thead>
                            <tr>
                                <th>Level</th>
                                <th>Damage</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${Object.keys(this.model.damage).map(key => {
                                return html`
                                    <tr>
                                        <td>${key}</td>
                                        <td>${this.model.damage[key]}</td>
                                    </tr>
                                `;
                            })}
                        </tbody>
                    </table>
                </div>
            `;
        } else {
            out = "";
        }
        return out;
    }

    override render(): void {
        const view = html`
            <div class="block w-full p-1">
                <div class="w-full mb-0.5" flex="row nowrap items-center justify-between">
                    <h1 class="block font-grey-800 font-lg font-bold">${this.model.name}</h1>
                    <div flex="row nowrap items-center">
                        ${new Button({
                            icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-edit" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M9 7h-3a2 2 0 0 0 -2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2 -2v-3"></path><path d="M9 15h3l8.5 -8.5a1.5 1.5 0 0 0 -3 -3l-8.5 8.5v3"></path><line x1="16" y1="5" x2="19" y2="8"></line></svg>`,
                            iconPosition: "center",
                            color: "grey",
                            kind: "text",
                            callback: this.editSpell.bind(this),
                            shape: "round",
                            size: "slim",
                            tooltip: "Edit"
                        })}
                        ${new Button({
                            icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-trash" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="4" y1="7" x2="20" y2="7"></line><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"></path><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"></path></svg>`,
                            iconPosition: "center",
                            color: "grey",
                            kind: "text",
                            callback: this.deleteSpell.bind(this),
                            shape: "round",
                            size: "slim",
                            tooltip: "Delete",
                        })}
                    </div>
                </div>
                <p class="block w-full font-grey-700 font-sm line-normal">${this.model.desc}</p>
            </div>
            <hr class="block w-full border-b-1 border-b-solid border-b-grey-200 my-1">
            <div class="details">
                <div>
                    <h3>Level</h3>
                    <p>${this.model.level || "Cantrip"}</p>
                </div>
                <div>
                    <h3>Casting Time</h3>
                    <p>${this.model.castingTime || "N/A"}</p>
                </div>
                <div>
                    <h3>Range/Area</h3>
                    <p>${this.model.range || "N/A"}</p>
                </div>
                <div>
                    <h3>Components</h3>
                    <p>${this.model.components.join(", ")} ${this.model.material?.length ? html`
                        <span tooltip="${this.model.material}" style="cursor: help;">*</span>
                    ` : ""}</p>
                </div>
                <div>
                    <h3>Duration</h3>
                    <p>${this.model.duration || "N/A"}</p>
                </div>
                <div>
                    <h3>Attack/Save</h3>
                    <p>${this.model.attackType || "N/A"}</p>
                </div>
                <div>
                    <h3>Damage/Effect</h3>
                    <p>${this.model.damageType || "N/A"}</p>
                </div>
                <div>
                    <h3>School</h3>
                    <p>${this.model.school || "N/A"}</p>
                </div>
                <div>
                    <h3>Classes</h3>
                    <p>${this.model.classes.join(", ")}</p>
                </div>
                <div>
                    <h3>Subclasses</h3>
                    <p>${this.model.subsclasses.join(", ")}</p>
                </div>
            </div>
            ${this.renderDamageTable()}
        `;
        render(view, this);
    }
}
env.bind("spell-component", Spell);