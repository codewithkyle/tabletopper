import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import { unsafeHTML } from "lit-html/directives/unsafe-html";
import env from "~brixi/controllers/env";
import { parseDataset } from "~brixi/utils/general";

type SaveKey = "str" | "dex" | "con" | "int" | "wis" | "cha";

interface ISaveRow {
    key: SaveKey;
    label: string;
}

interface ISavingThrowsTableModel {
    label: string;
    name: string; // form input name prefix
    // optional initial values: { str: 5, dex: 2, ... }
    values: Partial<Record<SaveKey, number>>;
}

const SAVES: ISaveRow[] = [
    { key: "str", label: "Strength" },
    { key: "dex", label: "Dexterity" },
    { key: "con", label: "Constitution" },
    { key: "int", label: "Intelligence" },
    { key: "wis", label: "Wisdom" },
    { key: "cha", label: "Charisma" },
];

class SavingThrowsTable extends SuperComponent<ISavingThrowsTableModel> {
    constructor() {
        super();
        this.model = {
            label: "Saving Throws",
            name: "saves",
            values: {},
        };
    }

    static get observedAttributes() {
        return ["data-label", "data-name", "data-values"];
    }

    override async connected() {
        await env.css(["saving-throws-table"]);
        const settings = parseDataset(this.dataset, this.model);
        settings.values = settings.values ?? {};
        this.set(settings);
    }

    private noopEvent: EventListener = (e: Event) => {
        e.stopImmediatePropagation();
    };

    private updateBonus: EventListener = (e: Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const key = input.dataset.key as SaveKey;

        const raw = input.value.trim();
        const next = raw === "" ? 0 : Number(raw);

        const updated = this.get();
        updated.values = { ...(updated.values || {}) };
        updated.values[key] = Number.isFinite(next) ? next : 0;

        this.set(updated, true);
    };

    override render(): void {
        const values = this.model.values || {};
        const heading = typeof this.model.label === "string" ? this.model.label.trim() : "";

        const view = html`
            ${heading
                ? html`<h4 class="block w-full font-medium font-sm font-grey-800 dark:font-grey-300 pl-0.125">
                      ${unsafeHTML(heading)}
                  </h4>`
                : null}

            <saves-grid>
                ${SAVES.map((save) => {
                    const bonus = values[save.key] ?? 0;
                    const inputName = `${this.model.name}-${save.key}`;

                    return html`
                        <save-row>
                            <save-name>
                                <span class="label">${save.label}</span>
                                <span class="meta">${save.key.toUpperCase()}</span>
                            </save-name>

                            <input
                                @keydown=${this.noopEvent}
                                @keyup=${this.noopEvent}
                                class="bonus"
                                type="number"
                                inputmode="numeric"
                                step="1"
                                name="${inputName}"
                                aria-label="${save.label} saving throw bonus"
                                .value="${String(bonus)}"
                                data-key="${save.key}"
                                @input=${this.updateBonus}
                            />
                        </save-row>
                    `;
                })}
            </saves-grid>
        `;

        render(view, this);
    }
}

env.bind("saving-throws-table", SavingThrowsTable);
