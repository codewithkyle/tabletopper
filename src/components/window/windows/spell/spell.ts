import db from "@codewithkyle/jsql";
import { publish } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import { unsafeHTML } from "lit-html/directives/unsafe-html";
import Button from "~brixi/components/buttons/button/button";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import type { Spell as S } from "~types/app";
import { marked } from "marked";
import Input from "~brixi/components/inputs/input/input";
import Textarea from "~brixi/components/textarea/textarea";
import NumberInput from "~brixi/components/inputs/number-input/number-input";

interface ISpell extends S{}
export default class Spell extends SuperComponent<ISpell>{

    constructor(index:string = null){
        super();
        this.stateMachine = {
            IDLING: {
                EDIT: "EDITING",
            },
            EDITING: {
                DONE: "IDLING",
            },
        };
        this.state = index ? "IDLING" : "EDITING";
        this.model = {
            index: index,
            name: "",
            desc: "",
            range: "",
            components: [],
            ritual: false,
            duration: "",
            castingTime: "",
            level: 0,
            attackType: "",
            damageType: "",
            damage: {},
            school: "",
            classes: [],
            subclasses: [],
            material: "",
            favorite: false,
        };
    }

    override async connected() {
        await env.css(["spell", "link"]);
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
        this.trigger("EDIT");
    }

    private toggleFavorite(){
        this.set({ favorite: !this.model.favorite });
        db.query("UPDATE spells SET $spell WHERE index = $index", {
            spell: {...this.model},
            index: this.model.index,
        });
    }

    private async saveSpell(){
        const isNew = this.model.index === null;
        if (this.model.index === null && this.model.name.trim().length === 0){
            notifications.error("Failed to save.", "Spells must have a name.");
            return;
        }
        const dmgRows:HTMLElement[] = Array.from(this.querySelectorAll("damage-table table-row"));
        let damage = {};
        for (let i = 0; i < dmgRows.length; i++){
            const key = dmgRows[i].querySelector("input:first-of-type").value.trim();
            const value = dmgRows[i].querySelector("input:last-of-type").value.trim();
            if (key.length && value.length){
                damage[key] = value;
            }
        }
        if (!Object.keys(damage).length){
            damage = null;
        }
        this.model.damage = damage;
        if (this.model.index === null){
            this.model.index = this.model.name.toLowerCase().trim().replace(/\s+/g, "-");
        }
        if (isNew){
            await db.query("INSERT INTO spells VALUES ($spell)", {
                spell: {...this.model},
            });
        } else {
            await db.query("UPDATE spells SET $spell WHERE index = $index", {
                spell: {...this.model},
                index: this.model.index,
            });
        }
        
        this.trigger("DONE");
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

    private renderIdling(){
        const markdown = marked.parse(this.model.desc);
        return html`
            <div class="block w-full p-1">
                <div class="w-full mb-0.5" flex="row nowrap items-center justify-between">
                    <h1 class="block font-grey-800 font-lg font-bold">${this.model.name}</h1>
                    <div flex="row nowrap items-center">
                        ${new Button({
                            icon: this.model.favorite ? `
                                    <svg class="font-warning-300" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                        <path d="M12 17.75l-6.172 3.245l1.179 -6.873l-5 -4.867l6.9 -1l3.086 -6.253l3.086 6.253l6.9 1l-5 4.867l1.179 6.873z" fill="currentColor"></path>
                                    </svg>
                                ` : `
                                    <svg class="font-grey-400" xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                        <path d="M12 17.75l-6.172 3.245l1.179 -6.873l-5 -4.867l6.9 -1l3.086 -6.253l3.086 6.253l6.9 1l-5 4.867l1.179 6.873z"></path>
                                    </svg>
                                `,
                            tooltip: this.model.favorite ? "Unfavorite" : "Mark as favorite",
                            kind: "text",
                            color: "grey",
                            iconPosition: "center",
                            shape: "round",
                            size: "slim",
                            callback: this.toggleFavorite.bind(this),
                        })}
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
                <div class="desc block w-full font-grey-700 font-sm line-normal">${unsafeHTML(markdown)}</div>
                ${this.model.material ? html`<p class="block w-full pt-1 font-sm font-grey-700 line-normal"><strong class="font-grey-800 font-medium">Required Materials:</strong><br>${this.model.material}</p>` : ""}
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
                    <p>${this.model.components?.join(", ")} ${this.model.material?.length ? html`
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
                    <p>${this.model.classes?.join(", ") ?? "N/A"}</p>
                </div>
                <div>
                    <h3>Subclasses</h3>
                    <p>${this.model.subclasses?.join(", ") ?? "N/A"}</p>
                </div>
            </div>
            ${this.renderDamageTable()}
        `;
    }

    private renderEditing(){
        return html`
            <div class="block w-full p-1">
                ${new Input({
                    name: "name",
                    label: "Spell",
                    value: this.model.name,
                    callback: (name) => {
                        this.set({
                            name: name,
                        }, true);
                    },
                    class: "mb-1.5",
                    required: true,
                })}
                ${new Textarea({
                    name: "description",
                    label: "Description <span class='font-sm font-grey-600'>(optional)</span>",
                    value: this.model.desc,
                    callback: (desc) => {
                        this.set({
                            desc: desc,
                        }, true);
                    },
                    class: "mb-1.5",
                    instructions: `This field supports <a class="link" href="https://www.markdownguide.org/cheat-sheet/" target="_blank">Markdown</a>.`,
                })}
                ${new Input({
                    name: "material",
                    value: this.model.material,
                    label: "Materials <span class='font-sm font-grey-600'>(optional)</span>",
                    callback: (value) => { this.set({ material: value }, true) },
                    class: "mb-1.5",
                })}
                <div grid="columns 2 gap-1.5" class="mb-1.5">
                    ${new NumberInput({
                        name: "level",
                        value: this.model.level || 0,
                        label: "Level <span class='font-sm font-grey-600'>(optional)</span>",
                        instructions: "Set Level to 0 for cantrips.",
                        callback: (value) => { this.set({ level: value }, true) },
                    })}
                    ${new Input({
                        name: "components",
                        value: this.model.components.join(", "),
                        label: "Components <span class='font-sm font-grey-600'>(optional)</span>",
                        instructions: "Components must be separated using commas.",
                        callback: (value) => {
                            const values = value.split(",");
                            const components = [];
                            for (let i = 0; i < values.length; i++){
                                components.push(values[i].trim());
                            }
                            this.set({
                                components: components,
                            }, true);
                        },
                    })}
                    ${new Input({
                        name: "castingTime",
                        value: this.model.castingTime,
                        label: "Casting Time <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ castingTime: value }, true) },
                    })}
                    ${new Input({
                        name: "range",
                        value: this.model.range,
                        label: "Range/Area <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ range: value }, true) },
                    })}
                    ${new Input({
                        name: "duration",
                        value: this.model.duration,
                        label: "Duration <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ duration: value }, true) },
                    })}
                    ${new Input({
                        name: "attackType",
                        value: this.model.attackType,
                        label: "Attack/Save <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ attackType: value }, true) },
                    })}
                    ${new Input({
                        name: "damage",
                        value: this.model.damageType,
                        label: "Damage/Effect <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ damageType: value }, true) },
                    })}
                    ${new Input({
                        name: "school",
                        value: this.model.school,
                        label: "School <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => { this.set({ school: value }, true) },
                    })}
                    ${new Input({
                        name: "classes",
                        value: this.model.classes.join(", "),
                        label: "Classes <span class='font-sm font-grey-600'>(optional)</span>",
                        instructions: "Class names must be separated using commas.",
                        callback: (value) => {
                            const values = value.split(",");
                            const classes = [];
                            for (let i = 0; i < values.length; i++){
                                classes.push(values[i].trim());
                            }
                            this.set({
                                classes: classes,
                            }, true);
                        },
                    })}
                    ${new Input({
                        name: "subclasses",
                        value: this.model.subclasses.join(", "),
                        instructions: "Subclass names must be separated using commas.",
                        label: "Subclasses <span class='font-sm font-grey-600'>(optional)</span>",
                        callback: (value) => {
                            const values = value.split(",");
                            const classes = [];
                            for (let i = 0; i < values.length; i++){
                                classes.push(values[i].trim());
                            }
                            this.set({
                                subclasses: classes,
                            }, true);
                        },
                    })}
                </div>
                <h4 class="block w-full font-medium font-sm font-grey-800 pl-0.125 mb-0.5">Damage <span class="font-sm font-grey-600">(optional)</span></h4>
                <damage-table class="mb-0.5">
                    ${Object.keys(this.model?.damage ?? {}).map((key, index) => {
                        return html`
                            <table-row>
                                <input required value="${key}" placeholder="Level">
                                <input required value="${this.model.damage[key]}" placeholder="Damage">
                            </table-row>
                        `;
                    })}
                </damage-table>
                ${new Button({
                    label: "Add Damage Row",
                    kind: "text",
                    color: "grey",
                    class: "w-full mb-1.5",
                    callback: ()=>{
                        const dmgTable = this.querySelector("damage-table");
                        if (dmgTable){
                            const newRow = document.createElement("table-row");
                            newRow.innerHTML = `
                                <input required placeholder="Level">
                                <input required placeholder="Damage">
                            `;
                            dmgTable.appendChild(newRow);
                        }
                    }
                })}
                ${new Button({
                    label: "Save Spell",
                    callback: this.saveSpell.bind(this),
                    kind: "solid",
                    color: "success",
                    class: "w-full",
                })}
            </div>
        `;
    }

    override render(): void {
        let view;
        switch(this.state){
            case "EDITING":
                view = this.renderEditing();
                break;
            case "IDLING":
                view = this.renderIdling();
                break;
            default:
                view = html`<p class="font-sm font-danger-700 block">Something went wrong.</p>`;
                break;
        }
        render(view, this);
    }
}
env.bind("spell-component", Spell);
