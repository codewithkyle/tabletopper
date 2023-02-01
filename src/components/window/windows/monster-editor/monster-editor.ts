import db from "@codewithkyle/jsql";
import { publish } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import Input from "~brixi/components/inputs/input/input";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import Select from "~brixi/components/select/select";
import env from "~brixi/controllers/env";
import notifications from "~brixi/controllers/notifications";
import { Ability, Monster } from "~types/app";

interface IMonsterEditor extends Monster{}
export default class MonsterEditor extends SuperComponent<IMonsterEditor>{
    constructor(index:string = null){
        super();
        this.model = {
            index: index,
            name: null,
            size: null,
            type:        null,
            subtype:     null,
            alignment:   null,
            ac:          null,
            hp:          null,
            hitDice:     null,
            str:         null,
            dex:         null,
            con:         null,
            int:         null,
            wis:         null,
            cha:         null,
            languages:   null,
            cr:          null,
            xp:          null,
            speed:       null,
            vulnerabilities:  null,
            resistances:      null,
            immunities:       null,
            senses:      null,
            savingThrows:null,
            skills:      null,
            abilities:      null,
            actions:        null,
            legendaryActions: null,
        };
    }

    override async connected(){
        await env.css(["monster-editor"]);
        const monster = (await db.query("SELECT * FROM monsters WHERE index = $index", { index: this.model.index }))?.[0] ?? null;
        if (monster !== null){
            this.set(monster, true);
        }
        this.render();
    }

    private saveCreature:EventListener = async (e:Event) => {
        e.preventDefault();
        e.stopImmediatePropagation();
        const target = e.currentTarget as HTMLElement;
        const data = {};
        let allValid = true;
        target.querySelectorAll(".js-input").forEach(el => {
            // @ts-expect-error
            const valid = el.validate();
            if (valid){
                // @ts-expect-error
                data[el.getName()] = el.getValue();
            } else {
                allValid = false;
            }
        });
        if (allValid){
            if (this.model.index !== null){
                data["index"] = this.model.index;
            } else {
                data["index"] = data["name"].replace(/[^a-z\s]/gi, "").trim().replace(/\s+/g, "-");
                this.set({
                    index: data["index"],
                }, true);
            }
            await db.query("INSERT INTO monsters VALUES ($monster)", {
                monster: data,
            });
            notifications.snackbar(`${data["name"]} has been updated.`);
            publish(data["index"], data);
        }
    }

    override render(): void {
        const sizes = [
            {
                label: "Tiny",
                value: "Tiny",
            },
            {
                label: "Small",
                value: "Small",
            },
            {
                label: "Medium",
                value: "Medium",
            },
            {
                label: "Large",
                value: "Large",
            },
            {
                label: "Huge",
                value: "Huge",
            },
            {
                label: "Gargantuan",
                value: "Gargantuan",
            },
        ];
        const alignments = [
            {
                label: "Unaligned",
                value: "unaliged",
            },
            {
                label: "Any Alignment",
                value: "any alignment",
            },
            {
                label: "Lawful Good",
                value: "lawful good",
            },
            {
                label: "Neutral Good",
                value: "neutral good",
            },
            {
                label: "Chaotic Good",
                value: "chaotic good",
            },
            {
                label: "Lawful Neutral",
                value: "lawful neutral",
            },
            {
                label: "Neutral",
                value: "neutral",
            },
            {
                label: "Chaotic Neutral",
                value: "chaotic neutral",
            },
            {
                label: "Lawful Evil",
                value: "lawful evil",
            },
            {
                label: "Neutral Evil",
                value: "neutral evil",
            },
            {
                label: "Chaotic Evil",
                value: "chaotic evil",
            },
            {
                label: "Any Neutral Alignment",
                value: "any neutral alignment",
            },
            {
                label: "Any Good Alignment",
                value: "any good alignment",
            },
            {
                label: "Any Chaotic Alignment",
                value: "any chaotic alignment",
            },
            {
                label: "Any Lawful Alignment",
                value: "any lawful alignment",
            },
            {
                label: "Any Non-Good Alignment",
                value: "any non-good alignment",
            },
        ];
        const types = [
            "aberration",
            "beast",
            "celestial",
            "construct",
            "dragon",
            "elemental",
            "fey",
            "fiend",
            "giant",
            "humanoid",
            "monstrositie",
            "ooze",
            "plant",
            "undead",
            "swam of tiny beasts",
        ];
        const subtypes = [
            "any race",
            "demon",
            "devil",
            "dwarf",
            "elf",
            "gnoll",
            "gnome",
            "goblinoid",
            "grimlock",
            "human",
            "kobold",
            "lizardfolk",
            "merfolk",
            "orc",
            "sahuagin",
            "shapechanger",
            "titan",
            "angel",
            "swarm",
        ];
        const view = html`
            <form @submit=${this.saveCreature} class="w-full p-1" grid="columns 1 gap-1">
                <div class="w-full" flex="row nowrap items-center">
                    ${new Input({
                        name: "name",
                        label: "Name",
                        value: this.model.name || "Unnamed Monster",
                        required: true,
                        css: "flex:1;",
                    })}
                </div>
                <div grid="columns 2 gap-1">
                    ${new Select({
                        name: "size",
                        label: "Size",
                        value: this.model.size,
                        options: sizes,
                        required: true,
                    })}
                    ${new Select({
                        name: "alignment",
                        label: "Alignment",
                        value: this.model.alignment,
                        options: alignments,
                    })}
                    ${new Input({
                        label: "Type",
                        name: "type",
                        value: this.model.type,
                        required: true,
                        datalist: types,
                    })}
                    ${new Input({
                        label: "Subtype",
                        name: "subtype",
                        value: this.model.subtype,
                        datalist: subtypes,
                    })}
                    ${new NumberInput({
                        label: "Armor Class",
                        name: "ac",
                        value: this.model.ac,
                        required: true,
                    })}
                    ${new NumberInput({
                        label: "Hit Points",
                        name: "hp",
                        value: this.model.hp,
                        required: true,
                    })}
                </div>
                ${new Input({
                    label: "Hit Dice",
                    name: "hitDice",
                    value: this.model.hitDice,
                })}
                ${new Input({
                    label: "Speed",
                    name: "speed",
                    value: this.model.speed,
                })}
                <div grid="columns 2 gap-1">
                    ${new NumberInput({
                        label: "Challenge Rating",
                        name: "cr",
                        value: this.model.cr,
                    })}
                    ${new NumberInput({
                        label: "XP",
                        name: "xp",
                        value: this.model.xp,
                        min: 0,
                        max: 9999999,
                    })}
                </div>
                <div grid="columns 3 gap-1">
                    ${new NumberInput({
                        label: "Strength",
                        name: "str",
                        value: this.model.str || 0,
                    })}
                    ${new NumberInput({
                        label: "Dexterity",
                        name: "dex",
                        value: this.model.dex || 0,
                    })}
                    ${new NumberInput({
                        label: "Constitution",
                        name: "con",
                        value: this.model.con || 0,
                    })}
                    ${new NumberInput({
                        label: "Intelligence",
                        name: "int",
                        value: this.model.int || 0,
                    })}
                    ${new NumberInput({
                        label: "Wisdom",
                        name: "wis",
                        value: this.model.wis || 0,
                    })}
                    ${new NumberInput({
                        label: "Charisma",
                        name: "cha",
                        value: this.model.cha || 0,
                    })}
                </div>
                ${new Input({
                    label: "Immunities",
                    name: "immunities",
                    value: this.model.immunities,
                })}
                ${new Input({
                    label: "Resistances",
                    name: "resistances",
                    value: this.model.resistances,
                })}
                ${new Input({
                    label: "Vulnerabilities",
                    name: "vulnerabilities",
                    value: this.model.vulnerabilities,
                })}
                ${new Input({
                    label: "Senses",
                    name: "senses",
                    value: this.model.senses,
                })}
                ${new Input({
                    label: "Languages",
                    name: "languages",
                    value: this.model.languages,
                })}
                ${new Input({
                    label: "Saving Throws",
                    name: "savingThrows",
                    value: this.model.savingThrows,
                })}
                ${new Input({
                    label: "Skills",
                    name: "skills",
                    value: this.model.skills,
                })}
                ${new MonsterInfoTable({
                    label: "Abilities",
                    name: "abilities",
                    rows: this.model.abilities || [],
                    addLabel: "Add Ability",
                    class: "mt-1",
                })}
                ${new MonsterInfoTable({
                    label: "Actions",
                    name: "actions",
                    rows: this.model.actions || [],
                    addLabel: "Add Action",
                    class: "mt-1",
                })}
                ${new MonsterInfoTable({
                    label: "Legendary Actions",
                    name: "legendaryActions",
                    rows: this.model.legendaryActions || [],
                    addLabel: "Add Legendary Action",
                    class: "mt-1",
                })}
                <div class="w-full">
                    <button class="bttn w-full" kind="solid" color="success">Save monster</button>
                </div>
            </form>
        `;
        render(view, this);
    }
}
env.bind("monster-editor", MonsterEditor);

interface IMonsterInfoTable {
    rows: Ability[],
    name: string,
    label: string,
    addLabel: string,
    css: string,
    class: string,
}
class MonsterInfoTable extends SuperComponent<IMonsterInfoTable>{

    constructor(settings = {}){
        super();
        this.model = {
            label: "",
            rows: [],
            name: "",
            addLabel: "Add Row",
            class: "",
            css: "",
        };
        this.model = Object.assign(this.model, settings);
    }

    override async connected(){
        await env.css(["monster-info-table", "buttons"]);
        this.render();
    }

    private addRow:EventListener = () => {
        const updatedModel = this.get();
        updatedModel.rows.push({
            name: "",
            desc: "",
        });
        this.set(updatedModel);
    }

    private deleteRow:EventListener = (e:Event) => {
        const button = e.currentTarget as HTMLElement;
        const index = parseInt(button.dataset.index);
        const updatedModel = this.get();
        updatedModel.rows.splice(index, 1);
        this.set(updatedModel);
    }

    public validate(){
        return true;
    }

    public getName():string{
        return this.model.name;
    }

    public getValue(){
        const output = [];
        const rows = Array.from(this.querySelectorAll("table-row"));
        for (let i = 0; i < rows.length; i++){
            const title = rows[i].querySelector("input");
            const description = rows[i].querySelector("textarea");
            output.push({
                name: title.value,
                desc: description.value
            });
        }
        return output;
    }

    private updateName:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLInputElement;
        const index = parseInt(target.dataset.index);
        const updatedModal = this.get();
        updatedModal.rows[index].name = target.value;
        this.set(updatedModal, true);
    }

    private updateDesc:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLTextAreaElement;
        const index = parseInt(target.dataset.index);
        const updatedModal = this.get();
        updatedModal.rows[index].desc = target.value;
        this.set(updatedModal, true);
    }

    override render(): void {
        console.log(this.model);
        this.style.cssText = this.model.css;
        this.className = `${this.model.class} js-input`;
        const view = html`
            <h4 class="block w-full font-medium font-sm font-grey-800 pl-0.125">${this.model.label}</h4>
            ${this.model.rows.map((row, index) => {
                return html`
                    <table-row>
                        <row-title>
                            <input required value="${row.name}" placeholder="Name" @input=${this.updateName} data-index="${index}">
                            <button aria-label="Delete ${row.name}" tooltip class="delete" type="button" @click=${this.deleteRow} data-index="${index}">
                                <svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path fill="currentColor" d="M296 432h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zm-160 0h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zM440 64H336l-33.6-44.8A48 48 0 0 0 264 0h-80a48 48 0 0 0-38.4 19.2L112 64H8a8 8 0 0 0-8 8v16a8 8 0 0 0 8 8h24v368a48 48 0 0 0 48 48h288a48 48 0 0 0 48-48V96h24a8 8 0 0 0 8-8V72a8 8 0 0 0-8-8zM171.2 38.4A16.1 16.1 0 0 1 184 32h80a16.1 16.1 0 0 1 12.8 6.4L296 64H152zM384 464a16 16 0 0 1-16 16H80a16 16 0 0 1-16-16V96h320zm-168-32h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8z"></path></svg>
                            </button>
                        </row-title>
                        <textarea required placeholder="Description" rows="4" @input=${this.updateDesc} data-index="${index}">${row.desc}</textarea>
                    </table-row>
                `;
            })}
            <button type="button" class="bttn" kind="text" color="grey" @click=${this.addRow}>${this.model.addLabel}</button>
        `;
        render(view, this);
    }
}
env.bind("monster-info-table", MonsterInfoTable);