import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import { unsafeHTML } from "lit-html/directives/unsafe-html";
import env from "~brixi/controllers/env";
import { parseDataset } from "~brixi/utils/general";

type SkillKey =
    | "acrobatics"
    | "animal_handling"
    | "arcana"
    | "athletics"
    | "deception"
    | "history"
    | "insight"
    | "intimidation"
    | "investigation"
    | "medicine"
    | "nature"
    | "perception"
    | "performance"
    | "persuasion"
    | "religion"
    | "sleight_of_hand"
    | "stealth"
    | "survival";

interface ISkillRow {
    key: SkillKey;
    label: string;
    ability: "STR" | "DEX" | "CON" | "INT" | "WIS" | "CHA";
    bonus: number;
}

interface ISkillsTableModel {
    label: string;
    name: string; // form input name prefix
    // optional initial values: { acrobatics: 3, ... }
    values: Partial<Record<SkillKey, number>>;
}

const SKILLS: Omit<ISkillRow, "bonus">[] = [
    { key: "acrobatics", label: "Acrobatics", ability: "DEX" },
    { key: "animal_handling", label: "Animal Handling", ability: "WIS" },
    { key: "arcana", label: "Arcana", ability: "INT" },
    { key: "athletics", label: "Athletics", ability: "STR" },
    { key: "deception", label: "Deception", ability: "CHA" },
    { key: "history", label: "History", ability: "INT" },
    { key: "insight", label: "Insight", ability: "WIS" },
    { key: "intimidation", label: "Intimidation", ability: "CHA" },
    { key: "investigation", label: "Investigation", ability: "INT" },
    { key: "medicine", label: "Medicine", ability: "WIS" },
    { key: "nature", label: "Nature", ability: "INT" },
    { key: "perception", label: "Perception", ability: "WIS" },
    { key: "performance", label: "Performance", ability: "CHA" },
    { key: "persuasion", label: "Persuasion", ability: "CHA" },
    { key: "religion", label: "Religion", ability: "INT" },
    { key: "sleight_of_hand", label: "Sleight of Hand", ability: "DEX" },
    { key: "stealth", label: "Stealth", ability: "DEX" },
    { key: "survival", label: "Survival", ability: "WIS" },
];

class SkillsTable extends SuperComponent<ISkillsTableModel> {
    constructor() {
        super();
        this.model = {
            label: "Skills",
            name: "skills",
            values: {},
        };
    }

    static get observedAttributes() {
        return ["data-label", "data-name", "data-values"];
    }

    override async connected() {
        await env.css(["skills-table"]);
        const settings = parseDataset(this.dataset, this.model);
        const maybeValues = settings.values;
        settings.values =
            maybeValues && typeof maybeValues === "object" && !Array.isArray(maybeValues)
                ? maybeValues
                : {};
        this.set(settings);
    }

    private noopEvent: EventListener = (e: Event) => {
        e.stopImmediatePropagation();
    };

    private updateBonus: EventListener = (e: Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const key = input.dataset.key as SkillKey;

        // Allow blank -> treat as 0, but keep UI stable.
        const raw = input.value.trim();
        const next = raw === "" ? 0 : Number(raw);

        const updated = this.get();
        updated.values = { ...(updated.values || {}) };
        updated.values[key] = Number.isFinite(next) ? next : 0;

        // re-render without blowing focus/typing too aggressively
        this.set(updated, true);
    };

    override render(): void {
        const values = this.model.values || {};

        const view = html`
            <h4 class="block w-full font-medium font-sm font-grey-800 dark:font-grey-300 pl-0.125">
                ${unsafeHTML(this.model.label)}
            </h4>

            <skills-grid>
                ${SKILLS.map((skill) => {
                    const bonus = values[skill.key] ?? 0;
                    const inputName = `${this.model.name}-${skill.key}`;

                    return html`
                        <skill-row>
                            <skill-name>
                                <span class="label">${skill.label}</span>
                                <span class="ability">${skill.ability}</span>
                            </skill-name>

                            <input
                                @keydown=${this.noopEvent}
                                @keyup=${this.noopEvent}
                                class="bonus"
                                type="number"
                                inputmode="numeric"
                                step="1"
                                name="${inputName}"
                                aria-label="${skill.label} bonus"
                                .value="${String(bonus)}"
                                data-key="${skill.key}"
                                @input=${this.updateBonus}
                            />
                        </skill-row>
                    `;
                })}
            </skills-grid>
        `;

        render(view, this);
    }
}

env.bind("skills-table", SkillsTable);
