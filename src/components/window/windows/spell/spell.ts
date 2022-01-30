import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";
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
                <h1 class="block font-grey-800 font-lg font-bold mb-0.5">${this.model.name}</h1>
                <p class="block w-full font-grey-700 font-sm line-normal">${this.model.desc}</p>
            </div>
            <hr class="block w-full border-b-1 border-b-solid border-b-grey-200 my-1">
            <div class="details">
                <div>
                    <h3>Level</h3>
                    <p>${this.model.level}</p>
                </div>
                <div>
                    <h3>Casting Time</h3>
                    <p>${this.model.castingTime}</p>
                </div>
                <div>
                    <h3>Range/Area</h3>
                    <p>${this.model.range}</p>
                </div>
                <div>
                    <h3>Components</h3>
                    <p>${this.model.components.join(", ")} ${this.model.material?.length ? html`
                        <span tooltip="${this.model.material}" style="cursor: help;">*</span>
                    ` : ""}</p>
                </div>
                <div>
                    <h3>Duration</h3>
                    <p>${this.model.duration}</p>
                </div>
                <div>
                    <h3>Attack/Save</h3>
                    <p>${this.model.attackType}</p>
                </div>
                <div>
                    <h3>Damage/Effect</h3>
                    <p>${this.model.damageType}</p>
                </div>
                <div>
                    <h3>School</h3>
                    <p>${this.model.school}</p>
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