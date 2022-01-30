import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Input from "~brixi/components/inputs/input/input";
import Select from "~brixi/components/select/select";
import env from "~brixi/controllers/env";
import { Spell as ISpell } from "~types/app";
import Window from "~components/window/window";
import Spell from "../spell/spell";
import { subscribe } from "@codewithkyle/pubsub";

interface ISpellbook{
    query: string,
    limit: number,
    classes: string[],
    level: string|number,
    castingTime: string,
    duration: string,
    range: string,
    damageType: string,
}
export default class Spellbook extends SuperComponent<ISpellbook>{
    constructor(){
        super();
        this.model = {
            query: "",
            limit: 10,
            classes: ["All"],
            level: null,
            castingTime: null,
            duration: null,
            range: null,
            damageType: null,
        };
        subscribe("spells", this.inbox.bind(this));
    }

    private inbox({ type, data }){
        switch(type){
            case "refresh":
                this.render();
                break;
            default:
                break;
        }
    }

    override async connected(){
        await env.css(["spellbook"]);
        this.render();
    }

    private showMore:EventListener = (e:Event) => {
        this.set({
            limit: this.model.limit + 10,
        });
    }

    private search(value){
        this.set({
            query: value,
        });
    }
    private debounceInput = this.debounce(this.search.bind(this), 300);

    private clickClassFilter:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        const className = target.dataset.class;
        console.log(className);
        const updatedModel = this.get();
        if (updatedModel.classes.includes(className)){
            const index = updatedModel.classes.indexOf(className);
            updatedModel.classes.splice(index, 1);
            if (updatedModel.classes.length === 0){
                updatedModel.classes.push("All");
            }
        } else {
            if (className === "All"){
                updatedModel.classes = ["All"];
            } else {
                updatedModel.classes.push(className);
                const i = updatedModel.classes.indexOf("All");
                if (i !== -1){
                    updatedModel.classes.splice(i, 1);
                }
            }
        }
        this.set(updatedModel);
    }

    private setLevel(level){
        this.set({
            level: level,
        });
    }

    private setCastingTime(time){
        this.set({
            castingTime: time,
        });
    }

    private setDuration(duration){
        this.set({
            duration: duration,
        });
    }

    private setDamageType(type){
        this.set({
            damageType: type,
        });
    }

    private setRange(range){
        this.set({
            range: range,
        });
    }

    private openSpell:EventListener = (e:Event) => {
        const target = e.currentTarget as HTMLElement;
        const window = document.body.querySelector(`window-component[window="${target.dataset.index}"]`) || new Window({
            name: target.dataset.name,
            view: new Spell(target.dataset.index),
        });
        if (!window.isConnected){
            document.body.append(window);
        }
    }

    private renderSpell(spell:ISpell, index): TemplateResult{
        return html`
            <button sfx="button" class="spell" @click=${this.openSpell} data-index="${spell.index}" data-name="${spell.name}">
                <div class="font-sm font-medium font-grey-800 bg-grey-50 radius-0.25 border-1 border-solid border-grey-100" style="width:32px;height:32px;" flex="column wrap items-center justify-center">
                    ${spell.level}
                </div>
                <div flex="column wrap" class="pl-1">
                    <h3 class="block font-medium font-grey-800 mb-0.25">${spell.name}</h3>
                    <h4 class="block font-xs font-grey-700">${spell.school} - ${spell.components.join(",")} ${spell.material?.length ? "*" : ""}</h4>
                </div>
                <span class="font-sm font-grey-700">${spell.castingTime}</span>
                <span class="font-sm font-grey-700">${spell.duration}</span>
                <span class="font-sm font-grey-700">${spell.range}</span>
                <span class="font-sm font-grey-700">${spell.damageType}</span>
            </button>
        `;
    }

    override async render(){
        let spells;
        let sql = "SELECT * FROM spells LIMIT $limit ";
        const bindings = {
            limit: this.model.limit,
        };
        const clauses = [];
        if (this.model.query?.length){
            clauses.push("name LIKE $query");
            bindings["query"] = this.model.query;
        }
        if (this.model.level !== null){
            clauses.push("level == $level");
            bindings["level"] = this.model.level;
        }
        if (this.model.classes[0] !== "All"){
            let classes = [];
            for (let i = 0; i < this.model.classes.length; i++){
                classes.push(`classes INCLUDES $class${i}`);
                bindings[`class${i}`] = this.model.classes[i];
            }
            clauses.push(classes.join(" OR "));
        }
        if (this.model.castingTime !== null){
            clauses.push("castingTime = $castTime");
            bindings["castTime"] = this.model.castingTime;
        }
        if (this.model.duration !== null){
            clauses.push("duration = $duration");
            bindings["duration"] = this.model.duration;
        }
        if (this.model.range !== null){
            clauses.push("range = $range");
            bindings["range"] = this.model.range;
        }
        if (this.model.damageType !== null){
            clauses.push("damageType = $dmgType");
            bindings["dmgType"] = this.model.damageType;
        }

        if (clauses.length){
            sql += `WHERE ${clauses.join(" AND ")}`;
        }
        spells = await db.query(sql, bindings);

        console.log(sql, bindings);


        const levels = await db.query("SELECT UNIQUE level FROM spells ORDER BY level");
        const levelOptions = [{
            label: "Any level",
            value: null,
        }];
        for (let i = 0; i < levels.length; i++){
            levelOptions.push({
                label: `Level ${levels[i]}`,
                value: levels[i],
            });
        }

        const castingTime = await db.query("SELECT UNIQUE castingTime FROM spells ORDER BY castingTime");
        const castingTimeOptions = [{
            label: "Any cast time",
            value: null,
        }];
        for (let i = 0; i < castingTime.length; i++){
            castingTimeOptions.push({
                label: castingTime[i],
                value: castingTime[i],
            });
        }

        const duration = await db.query("SELECT UNIQUE duration FROM spells ORDER BY duration");
        const durationOptions = [{
            label: "Any duration",
            value: null,
        }];
        for (let i = 0; i < duration.length; i++){
            durationOptions.push({
                label: duration[i],
                value: duration[i],
            });
        }

        const range = await db.query("SELECT UNIQUE range FROM spells ORDER BY range");
        const rangeOptions = [{
            label: "Any range",
            value: null,
        }];
        for (let i = 0; i < range.length; i++){
            rangeOptions.push({
                label: range[i],
                value: range[i],
            });
        }

        const damageType = await db.query("SELECT UNIQUE damageType FROM spells ORDER BY damageType");
        const damageTypeOptions = [{
            label: "Any damage type",
            value: null,
        }];
        for (let i = 0; i < damageType.length; i++){
            if (damageType[i] !== null){
                damageTypeOptions.push({
                    label: damageType[i],
                    value: damageType[i],
                });
            }
        }

        const view = html`
            <div class="filters">
                <div class="w-full scroll-x-auto">
                    <div class="classes">
                        <button sfx="button" class="${this.model.classes.includes("All") ? "is-selected" : ""}" aria-label="All Spells" tooltip data-class="All" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="hand-holding-magic" class="svg-inline--fa fa-hand-holding-magic fa-w-18" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 576 512"><path fill="currentColor" d="M224 224c52.94 0 96-43.06 96-96 0 0-32 32-96 32-17.72 0-32-17.5-32-32V96c0-17.64 14.36-32 32-32h128c17.64 0 32 14.36 32 32v32c0 34.3-17.51 66.54-42.09 90.8L288 272l17.81-1.01c77.05-4.36 141.58-65.27 142.18-141.43 0-.52.01-1.04.01-1.56V96c0-52.94-43.06-96-96-96H224c-52.94 0-96 43.06-96 96v32c0 51.14 44.86 96 96 96zm341.28 104.1c-11.83-10.69-30.18-9.96-42.63 0l-92.34 73.87A64.004 64.004 0 0 1 390.33 416H272c-8.84 0-16-7.16-16-16s7.16-16 16-16h78.28c15.94 0 30.72-10.89 33.28-26.63C386.82 337.32 371.43 320 352 320H192c-26.97 0-53.11 9.27-74.06 26.27L71.44 384H16c-8.84 0-16 7.16-16 16v96c0 8.84 7.16 16 16 16h356.77c14.53 0 28.63-4.95 39.98-14.03l151.23-120.99c15.19-12.13 16.38-35.27 1.3-48.88z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Artificer") ? "is-selected" : ""}" aria-label="Artificer" tooltip data-class="Artificer" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="flask-poison" class="svg-inline--fa fa-flask-poison fa-w-13" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 416 512"><path fill="currentColor" d="M80 48V16c0-8.84 7.16-16 16-16h224c8.84 0 16 7.16 16 16v32c0 8.84-7.16 16-16 16H96c-8.84 0-16-7.16-16-16zm88 240c-13.25 0-24 10.74-24 24 0 13.25 10.75 24 24 24s24-10.75 24-24c0-13.26-10.75-24-24-24zm247.95 68.67c-1.14 51.68-21.15 98.7-53.39 134.48-12.09 13.41-29.52 20.85-47.58 20.85H100.94c-17.78 0-35.05-7.13-47-20.3C20.43 454.79 0 405.79 0 352c0-80.12 45.61-149.15 112-183.88V96h192v73.05c67.19 36.13 113.71 107.67 111.95 187.62zM320 312c0-48.6-50.14-88-112-88S96 263.4 96 312c0 29.87 19.04 56.17 48 72.08V416c0 8.84 7.16 16 16 16h96c8.84 0 16-7.16 16-16v-31.92c28.96-15.91 48-42.21 48-72.08zm-72-24c-13.25 0-24 10.74-24 24 0 13.25 10.75 24 24 24s24-10.75 24-24c0-13.26-10.75-24-24-24z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Bard") ? "is-selected" : ""}" aria-label="Bard" tooltip data-class="Bard" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="mandolin" class="svg-inline--fa fa-mandolin fa-w-16" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path fill="currentColor" d="M507.31 50.67l-46-46a16 16 0 0 0-19.81-2.25l-64 40.07a32 32 0 0 0-14 19.11L349 117.73l-74.46 74.46C90.38 190.67 46 241 30.74 256.28c-46.81 46.79-39.52 150.36 17.54 207.45s160.65 64.34 207.44 17.53C271 466 321.32 421.62 319.8 237.45L394.27 163l56.11-14.51a32 32 0 0 0 19.11-14l40.07-64a16 16 0 0 0-2.25-19.82zM208 352a48 48 0 1 1 48-48 48 48 0 0 1-48 48z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Cleric") ? "is-selected" : ""}" aria-label="Cleric" tooltip data-class="Cleric" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="star-of-life" class="svg-inline--fa fa-star-of-life fa-w-15" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 480 512"><path fill="currentColor" d="M471.99 334.43L336.06 256l135.93-78.43c7.66-4.42 10.28-14.2 5.86-21.86l-32.02-55.43c-4.42-7.65-14.21-10.28-21.87-5.86l-135.93 78.43V16c0-8.84-7.17-16-16.01-16h-64.04c-8.84 0-16.01 7.16-16.01 16v156.86L56.04 94.43c-7.66-4.42-17.45-1.79-21.87 5.86L2.15 155.71c-4.42 7.65-1.8 17.44 5.86 21.86L143.94 256 8.01 334.43c-7.66 4.42-10.28 14.21-5.86 21.86l32.02 55.43c4.42 7.65 14.21 10.27 21.87 5.86l135.93-78.43V496c0 8.84 7.17 16 16.01 16h64.04c8.84 0 16.01-7.16 16.01-16V339.14l135.93 78.43c7.66 4.42 17.45 1.8 21.87-5.86l32.02-55.43c4.42-7.65 1.8-17.43-5.86-21.85z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Druid") ? "is-selected" : ""}" aria-label="Druid" tooltip data-class="Druid" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="scythe" class="svg-inline--fa fa-scythe fa-w-20" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 512"><path fill="currentColor" d="M608 0h-25.45l-59.74 288H400a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h109.54l-29.26 141.05A16 16 0 0 0 496 512h31.45a16 16 0 0 0 15.72-13l96.27-461A32 32 0 0 0 608 0zm-58.13 0h-211C192 0 64 64 0 192h510z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Paladin") ? "is-selected" : ""}" aria-label="Paladin" tooltip data-class="Paladin" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="helmet-battle" class="svg-inline--fa fa-helmet-battle fa-w-18" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 576 512"><path fill="currentColor" d="M32.01 256C49.68 256 64 243.44 64 227.94V0L.97 221.13C-4.08 238.84 11.2 256 32.01 256zm543.02-34.87L512 0v227.94c0 15.5 14.32 28.06 31.99 28.06 20.81 0 36.09-17.16 31.04-34.87zM480 210.82C480 90.35 288 0 288 0S96 90.35 96 210.82c0 82.76-22.86 145.9-31.13 180.71-3.43 14.43 3.59 29.37 16.32 35.24L256 512V256l-96-32v-32h256v32l-96 32v256l174.82-85.23c12.73-5.87 19.75-20.81 16.32-35.24-8.28-34.81-31.14-97.95-31.14-180.71z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Ranger") ? "is-selected" : ""}" aria-label="Ranger" tooltip data-class="Ranger" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="bow-arrow" class="svg-inline--fa fa-bow-arrow fa-w-16" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path fill="currentColor" d="M145.78 287.03l45.26-45.25-90.58-90.58C128.24 136.08 159.49 128 192 128c32.03 0 62.86 7.79 90.33 22.47l46.61-46.61C288.35 78.03 241.3 64 192 64c-49.78 0-97.29 14.27-138.16 40.59l-3.9-3.9c-6.25-6.25-16.38-6.25-22.63 0L4.69 123.31c-6.25 6.25-6.25 16.38 0 22.63l141.09 141.09zm262.36-104.64L361.53 229c14.68 27.47 22.47 58.3 22.47 90.33 0 32.51-8.08 63.77-23.2 91.55l-90.58-90.58-45.26 45.26 141.76 141.76c6.25 6.25 16.38 6.25 22.63 0l22.63-22.63c6.25-6.25 6.25-16.38 0-22.63l-4.57-4.57C433.74 416.63 448 369.11 448 319.33c0-49.29-14.03-96.35-39.86-136.94zM493.22.31L364.63 26.03c-12.29 2.46-16.88 17.62-8.02 26.49l34.47 34.47-250.64 250.63-49.7-16.57a20.578 20.578 0 0 0-21.04 4.96L6.03 389.69c-10.8 10.8-6.46 29.2 8.04 34.04l55.66 18.55 18.55 55.65c4.83 14.5 23.23 18.84 34.04 8.04l63.67-63.67a20.56 20.56 0 0 0 4.97-21.04l-16.57-49.7 250.64-250.64 34.47 34.47c8.86 8.86 24.03 4.27 26.49-8.02l25.72-128.59C513.88 7.8 504.2-1.88 493.22.31z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Sorcerer") ? "is-selected" : ""}" aria-label="Sorcerer" tooltip data-class="Sorcerer" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fab" data-icon="gripfire" class="svg-inline--fa fa-gripfire fa-w-12" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 384 512"><path fill="currentColor" d="M112.5 301.4c0-73.8 105.1-122.5 105.1-203 0-47.1-34-88-39.1-90.4.4 3.3.6 6.7.6 10C179.1 110.1 32 171.9 32 286.6c0 49.8 32.2 79.2 66.5 108.3 65.1 46.7 78.1 71.4 78.1 86.6 0 10.1-4.8 17-4.8 22.3 13.1-16.7 17.4-31.9 17.5-46.4 0-29.6-21.7-56.3-44.2-86.5-16-22.3-32.6-42.6-32.6-69.5zm205.3-39c-12.1-66.8-78-124.4-94.7-130.9l4 7.2c2.4 5.1 3.4 10.9 3.4 17.1 0 44.7-54.2 111.2-56.6 116.7-2.2 5.1-3.2 10.5-3.2 15.8 0 20.1 15.2 42.1 17.9 42.1 2.4 0 56.6-55.4 58.1-87.7 6.4 11.7 9.1 22.6 9.1 33.4 0 41.2-41.8 96.9-41.8 96.9 0 11.6 31.9 53.2 35.5 53.2 1 0 2.2-1.4 3.2-2.4 37.9-39.3 67.3-85 67.3-136.8 0-8-.7-16.2-2.2-24.6z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Warlock") ? "is-selected" : ""}" aria-label="Warlock" tooltip data-class="Warlock" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="swords" class="svg-inline--fa fa-swords fa-w-16" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path fill="currentColor" d="M309.37 389.38l80-80L93.33 13.33 15.22.14C6.42-1.12-1.12 6.42.14 15.22l13.2 78.11 296.03 296.05zm197.94 72.68L448 402.75l31.64-59.03c3.33-6.22 2.2-13.88-2.79-18.87l-17.54-17.53c-6.25-6.25-16.38-6.25-22.63 0L307.31 436.69c-6.25 6.25-6.25 16.38 0 22.62l17.53 17.54a16 16 0 0 0 18.87 2.79L402.75 448l59.31 59.31c6.25 6.25 16.38 6.25 22.63 0l22.62-22.62c6.25-6.25 6.25-16.38 0-22.63zm-8.64-368.73l13.2-78.11c1.26-8.8-6.29-16.34-15.08-15.08l-78.11 13.2-140.05 140.03 80 80L498.67 93.33zm-345.3 185.3L100 332l-24.69-24.69c-6.25-6.25-16.38-6.25-22.62 0l-17.54 17.53a15.998 15.998 0 0 0-2.79 18.87L64 402.75 4.69 462.06c-6.25 6.25-6.25 16.38 0 22.63l22.62 22.62c6.25 6.25 16.38 6.25 22.63 0L109.25 448l59.03 31.64c6.22 3.33 13.88 2.2 18.87-2.79l17.53-17.54c6.25-6.25 6.25-16.38 0-22.62L180 412l53.37-53.37-80-80z"></path></svg>
                        </button>
                        <button sfx="button" class="${this.model.classes.includes("Wizard") ? "is-selected" : ""}" aria-label="Wizard" tooltip data-class="Wizard" @click=${this.clickClassFilter}>
                            <svg aria-hidden="true" focusable="false" data-prefix="fas" data-icon="hat-wizard" class="svg-inline--fa fa-hat-wizard fa-w-16" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><path fill="currentColor" d="M496 448H16c-8.84 0-16 7.16-16 16v32c0 8.84 7.16 16 16 16h480c8.84 0 16-7.16 16-16v-32c0-8.84-7.16-16-16-16zm-304-64l-64-32 64-32 32-64 32 64 64 32-64 32-16 32h208l-86.41-201.63a63.955 63.955 0 0 1-1.89-45.45L416 0 228.42 107.19a127.989 127.989 0 0 0-53.46 59.15L64 416h144l-16-32zm64-224l16-32 16 32 32 16-32 16-16 32-16-32-32-16 32-16z"></path></svg>
                        </button>
                    </div>
                </div>
                <div class="inputs">
                    ${new Input({
                        name: "spellSearch",
                        value: this.model.query,
                        placeholder: "Search spells...",
                        callback: this.debounceInput.bind(this),
                    })}
                    ${new Select({
                        name: "spellLevel",
                        options: levelOptions,
                        value: this.model.level,
                        callback: this.setLevel.bind(this),
                    })}
                    ${new Select({
                        name: "spellCastingTime",
                        options: castingTimeOptions,
                        value: this.model.castingTime,
                        callback: this.setCastingTime.bind(this),
                    })}
                    ${new Select({
                        name: "spellDuration",
                        options: durationOptions,
                        value: this.model.duration,
                        callback: this.setDuration.bind(this),
                    })}
                    ${new Select({
                        name: "spellRange",
                        options: rangeOptions,
                        value: this.model.range,
                        callback: this.setRange.bind(this),
                    })}
                    ${new Select({
                        name: "spellDamageType",
                        options: damageTypeOptions,
                        value: this.model.damageType,
                        callback: this.setDamageType.bind(this),
                    })}
                </div>
            </div>
            <div class="w-full block border-t-1 border-t-solid border-t-grey-200">
                <div class="spells">
                    <div class="heading">
                        <span>Level</span>
                        <span class="pl-1">Spell</span>
                        <span>Cast Time</span>
                        <span>Duration</span>
                        <span>Range</span>
                        <span>Damage/Effect</span>
                    </div>
                    ${spells.map(this.renderSpell.bind(this))}
                </div>
                <div class="w-full px-1 pb-1">
                    <button @click=${this.showMore} class="w-full bttn" color="grey" kind="text" sfx="button">load more spells</button>
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("spell-book", Spellbook);