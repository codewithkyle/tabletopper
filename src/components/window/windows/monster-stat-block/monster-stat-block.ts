import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import env from "~brixi/controllers/env";
import { Monster } from "~types/app";
import { CalculateModifier, CalculateProficiencyBonus } from "~utils/game";

interface IMonsterStatBlock extends Monster {};
export default class MonsterStatBlock extends SuperComponent<IMonsterStatBlock>{
    constructor(index:string){
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
        await env.css(["monster-stat-block"]);
        const monster = (await db.query("SELECT * FROM monsters WHERE index = $index", { index: this.model.index }))[0];
        this.set(monster);
    }

    override render(): void {
        const view = html`
            <div class="container">
                <div class="stats line-normal">
                    <div class="block w-full p-0.5">
                        <h3 class="block font-danger-800 font-lg font-bold font-serif">${this.model.name}</h3>
                        <p style="font-style:italic;" class="block font-neutral-900 font-xs">${this.model.size}${this.model.type ? ` ${this.model.type}` : null}${this.model.subtype ? ` ${this.model.subtype}` : null}, ${this.model.alignment}</p>
                    </div>
                    <hr>
                    <p class="block w-full p-0.5 font-danger-800 font-sm">
                        <strong>Armor Class</strong>
                        ${this.model.ac}
                        <br>
                        <strong>Hit Points</strong>
                        ${this.model.hp} ${this.model.hitDice ? `(${this.model.hitDice})` : null}
                        <br>
                        <strong>Speed</strong>
                        ${this.model.speed}
                        <br>
                        <strong>Challenge</strong>
                        ${this.model.cr} (${this.model.xp} XP)
                    </p>
                    <hr>
                    <div class="abilities p-0.5">
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">STR</strong>
                            <span>${this.model.str} (${CalculateModifier(this.model.str)})</span>
                        </div>
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">DEX</strong>
                            <span>${this.model.dex} (${CalculateModifier(this.model.dex)})</span>
                        </div>
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">CON</strong>
                            <span>${this.model.con} (${CalculateModifier(this.model.con)})</span>
                        </div>
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">INT</strong>
                            <span>${this.model.int} (${CalculateModifier(this.model.int)})</span>
                        </div>
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">WIS</strong>
                            <span>${this.model.wis} (${CalculateModifier(this.model.wis)})</span>
                        </div>
                        <div class="text-center font-danger-800 font-sm">
                            <strong class="block">CHA</strong>
                            <span>${this.model.cha} (${CalculateModifier(this.model.cha)})</span>
                        </div>
                    </div>
                    <hr>
                    <p class="block w-full p-0.5 font-danger-800 font-sm">
                        <strong>Immunities</strong>
                        ${this.model.immunities || "-"}
                        <br>
                        <strong>Resistances</strong>
                        ${this.model.resistances || "-"}
                        <br>
                        <strong>Vulnerabilities</strong>
                        ${this.model.vulnerabilities || "-"}
                        <br>
                        <strong>Senses</strong>
                        ${this.model.senses || "-"}
                        <br>
                        <strong>Languages</strong>
                        ${this.model.languages || "—"}
                        <br>
                        <strong>Saving Throws</strong>
                        ${this.model.savingThrows || "-"}
                        <br>
                        <strong>Skills</strong>
                        ${this.model.skills || "-"}
                        <br>
                        <strong>Proficiency Bonus</strong>
                        ${CalculateProficiencyBonus(this.model.cr)}
                        <br>
                    </p>
                    <hr>
                    ${this.model.abilities.length ? html`
                        <dl class="block w-full p-0.5 font-sm font-neutral-900">
                            ${this.model.abilities.map((ability) => {
                                return html`
                                    <dt class="block mt-0.5 font-bold block">${ability.name}</dt>
                                    <dd>${ability.desc}</dd>
                                `;
                            })}
                        </dl>
                    ` : null}
                    ${this.model.actions.length ? html`
                        <h4 class="pb-0.5 pt-1 block font-danger-800 font-md border-b-1 border-b-solid border-b-danger-800 mx-auto" style="width:calc(100% - 1rem);">Actions</h4>
                        <dl class="block w-full p-0.5 font-sm font-neutral-900">
                            ${this.model.actions.map((action) => {
                                return html`
                                    <dt class="block mt-0.5 font-bold block">${action.name}</dt>
                                    <dd>${action.desc}</dd>
                                `;
                            })}
                        </dl>
                    ` : null}
                    ${this.model.legendaryActions.length ? html`
                        <h4 class="pb-0.5 pt-1 block font-danger-800 font-md border-b-1 border-b-solid border-b-danger-800 mx-auto" style="width:calc(100% - 1rem);">Legendary Actions</h4>
                        <dl class="block w-full p-0.5 font-sm font-neutral-900">
                            ${this.model.legendaryActions.map((action) => {
                                return html`
                                    <dt class="block mt-0.5 font-bold block">${action.name}</dt>
                                    <dd>${action.desc}</dd>
                                `;
                            })}
                        </dl>
                    ` : null}
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("monster-stat-block", MonsterStatBlock);