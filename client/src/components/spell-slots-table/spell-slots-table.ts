// spell-slots-table.ts
import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import { unsafeHTML } from "lit-html/directives/unsafe-html";
import env from "~brixi/controllers/env";
import { parseDataset } from "~brixi/utils/general";

type SpellLevel = 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9;

type SpellSchool =
    | "Abjuration"
    | "Conjuration"
    | "Divination"
    | "Enchantment"
    | "Evocation"
    | "Illusion"
    | "Necromancy"
    | "Transmutation";

interface ISpell {
    name: string;
    components: string;
    school: SpellSchool;
    castingTime: string;
    range: string;
    duration: string;
    text: string;
}

interface ISpellLevelModel {
    level: SpellLevel;
    slots: number;
    used: number;
    spells: ISpell[];
}

interface ISpellSlotsTableModel {
    label: string;
    name: string; // input name prefix
    addLabel: string;
    levels: Record<string, ISpellLevelModel>; // keys: "0".."9"
}

const SCHOOLS: SpellSchool[] = [
    "Abjuration",
    "Conjuration",
    "Divination",
    "Enchantment",
    "Evocation",
    "Illusion",
    "Necromancy",
    "Transmutation",
];

const LEVELS: SpellLevel[] = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

const levelLabel = (lvl: SpellLevel) => (lvl === 0 ? "Cantrips" : `Level ${lvl}`);

const blankSpell = (): ISpell => ({
    name: "",
    components: "",
    school: "Evocation",
    castingTime: "",
    range: "",
    duration: "",
    text: "",
});

const clampInt = (n: number, min = 0, max = 99) => {
    if (!Number.isFinite(n)) return min;
    const x = Math.trunc(n);
    return Math.max(min, Math.min(max, x));
};

class SpellSlotsTable extends SuperComponent<ISpellSlotsTableModel> {
    constructor() {
        super();
        const levels: Record<string, ISpellLevelModel> = {};
        LEVELS.forEach((lvl) => {
            levels[String(lvl)] = {
                level: lvl,
                slots: 0,
                used: 0,
                spells: [],
            };
        });

        this.model = {
            label: "Spell Slots",
            name: "spells",
            addLabel: "Add Spell",
            levels,
        };
    }

    static get observedAttributes() {
        return ["data-label", "data-name", "data-add-label", "data-levels"];
    }

    override async connected() {
        await env.css(["spell-slots-table"]);
        const settings = parseDataset(this.dataset, this.model);
        const incomingLevels =
            settings.levels && typeof settings.levels === "object" && !Array.isArray(settings.levels)
                ? settings.levels
                : {};

        // Ensure levels object exists and contains 0..9 (merge defaults).
        const merged: ISpellSlotsTableModel = {
            ...this.model,
            ...settings,
            levels: { ...(this.model.levels || {}), ...incomingLevels },
        };

        LEVELS.forEach((lvl) => {
            const key = String(lvl);
            const existing = merged.levels[key];
            merged.levels[key] = {
                level: lvl,
                slots: clampInt(existing?.slots ?? 0),
                used: clampInt(existing?.used ?? 0),
                spells: Array.isArray(existing?.spells)
                    ? existing.spells.map((spell) => ({
                          name: typeof spell?.name === "string" ? spell.name : "",
                          components: typeof spell?.components === "string" ? spell.components : "",
                          school:
                              typeof spell?.school === "string" &&
                              SCHOOLS.includes(spell.school as SpellSchool)
                                  ? (spell.school as SpellSchool)
                                  : "Evocation",
                          castingTime: typeof spell?.castingTime === "string" ? spell.castingTime : "",
                          range: typeof spell?.range === "string" ? spell.range : "",
                          duration: typeof spell?.duration === "string" ? spell.duration : "",
                          text: typeof spell?.text === "string" ? spell.text : "",
                      }))
                    : [],
            };
        });

        this.set(merged);
    }

    private noopEvent: EventListener = (e: Event) => {
        e.stopImmediatePropagation();
    };

    private updateSlots: EventListener = (e: Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const lvl = parseInt(input.dataset.level) as SpellLevel;
        const raw = input.value.trim();
        const next = raw === "" ? 0 : clampInt(Number(raw));

        const updated = this.get();
        const key = String(lvl);
        const level = updated.levels[key];

        level.slots = next;
        // keep used <= slots
        level.used = clampInt(level.used, 0, level.slots);

        this.set(updated, true);
    };

    private updateUsed: EventListener = (e: Event) => {
        const input = e.currentTarget as HTMLInputElement;
        const lvl = parseInt(input.dataset.level) as SpellLevel;
        const raw = input.value.trim();
        const next = raw === "" ? 0 : clampInt(Number(raw));

        const updated = this.get();
        const key = String(lvl);
        const level = updated.levels[key];

        level.used = clampInt(next, 0, level.slots);

        this.set(updated, true);
    };

    private addSpell: EventListener = (e: Event) => {
        const button = e.currentTarget as HTMLElement;
        const lvl = parseInt(button.dataset.level) as SpellLevel;

        const updated = this.get();
        updated.levels[String(lvl)].spells.push(blankSpell());
        this.set(updated);
    };

    private deleteSpell: EventListener = (e: Event) => {
        const button = e.currentTarget as HTMLElement;
        const lvl = parseInt(button.dataset.level) as SpellLevel;
        const index = parseInt(button.dataset.index);

        const updated = this.get();
        updated.levels[String(lvl)].spells.splice(index, 1);
        this.set(updated);
    };

    private updateSpellField: EventListener = (e: Event) => {
        const el = e.currentTarget as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
        const lvl = parseInt((el.dataset.level as string) || "0") as SpellLevel;
        const index = parseInt((el.dataset.index as string) || "0");
        const field = el.dataset.field as keyof ISpell;

        const updated = this.get();
        const spell = updated.levels[String(lvl)].spells[index];

        if (!spell) return;

        // keep it simple: inputs/textarea/select all use .value
        (spell[field] as any) = el.value;

        this.set(updated, true);
    };

    override render(): void {
        const view = html`
            <h4 class="block w-full font-medium font-sm font-grey-800 dark:font-grey-300 pl-0.125">
                ${unsafeHTML(this.model.label)}
            </h4>

            <levels-grid>
                ${LEVELS.map((lvl) => {
                    const level = this.model.levels[String(lvl)];

                    const slotsName = `${this.model.name}-level-${lvl}-slots`;
                    const usedName = `${this.model.name}-level-${lvl}-used`;

                    return html`
                        <spell-level>
                            <level-header>
                                <div class="title">
                                    <span class="lvl">${levelLabel(lvl)}</span>
                                </div>

                                <div class="slots">
                                    <label>
                                        <span class="k">Slots</span>
                                        <input
                                            @keydown=${this.noopEvent}
                                            @keyup=${this.noopEvent}
                                            type="number"
                                            step="1"
                                            min="0"
                                            name="${slotsName}"
                                            .value="${String(level.slots ?? 0)}"
                                            data-level="${lvl}"
                                            @input=${this.updateSlots}
                                        />
                                    </label>

                                    <label>
                                        <span class="k">Used</span>
                                        <input
                                            @keydown=${this.noopEvent}
                                            @keyup=${this.noopEvent}
                                            type="number"
                                            step="1"
                                            min="0"
                                            name="${usedName}"
                                            .value="${String(level.used ?? 0)}"
                                            data-level="${lvl}"
                                            @input=${this.updateUsed}
                                        />
                                    </label>
                                </div>
                            </level-header>

                            <spells-list>
                                ${level.spells.map((spell, index) => {
                                    const base = `${this.model.name}-level-${lvl}-spell-${index}`;
                                    return html`
                                        <spell-card>
                                            <spell-top>
                                                <div class="name">
                                                    <input
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        type="text"
                                                        required
                                                        placeholder="Spell name"
                                                        name="${base}-name"
                                                        .value="${spell.name}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="name"
                                                        @input=${this.updateSpellField}
                                                    />
                                                </div>

                                                <button
                                                    class="delete"
                                                    type="button"
                                                    aria-label="Delete ${spell.name || "spell"}"
                                                    tooltip
                                                    data-level="${lvl}"
                                                    data-index="${index}"
                                                    @click=${this.deleteSpell}
                                                >
                                                    <svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512">
                                                        <path fill="currentColor" d="M296 432h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zm-160 0h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zM440 64H336l-33.6-44.8A48 48 0 0 0 264 0h-80a48 48 0 0 0-38.4 19.2L112 64H8a8 8 0 0 0-8 8v16a8 8 0 0 0 8 8h24v368a48 48 0 0 0 48 48h288a48 48 0 0 0 48-48V96h24a8 8 0 0 0 8-8V72a8 8 0 0 0-8-8zM171.2 38.4A16.1 16.1 0 0 1 184 32h80a16.1 16.1 0 0 1 12.8 6.4L296 64H152zM384 464a16 16 0 0 1-16 16H80a16 16 0 0 1-16-16V96h320zm-168-32h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8z"></path>
                                                    </svg>
                                                </button>
                                            </spell-top>

                                            <spell-grid>
                                                <label class="full">
                                                    <span class="k">Components</span>
                                                    <input
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        type="text"
                                                        placeholder="V, S, M (a bit of sponge)"
                                                        name="${base}-components"
                                                        .value="${spell.components}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="components"
                                                        @input=${this.updateSpellField}
                                                    />
                                                </label>

                                                <label>
                                                    <span class="k">School</span>
                                                    <select
                                                        name="${base}-school"
                                                        .value="${spell.school}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="school"
                                                        @change=${this.updateSpellField}
                                                    >
                                                        ${SCHOOLS.map((s) => html`<option value="${s}">${s}</option>`)}
                                                    </select>
                                                </label>

                                                <label>
                                                    <span class="k">Cast Time</span>
                                                    <input
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        type="text"
                                                        placeholder="Action"
                                                        name="${base}-castingTime"
                                                        .value="${spell.castingTime}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="castingTime"
                                                        @input=${this.updateSpellField}
                                                    />
                                                </label>

                                                <label>
                                                    <span class="k">Range</span>
                                                    <input
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        type="text"
                                                        placeholder="150 feet"
                                                        name="${base}-range"
                                                        .value="${spell.range}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="range"
                                                        @input=${this.updateSpellField}
                                                    />
                                                </label>

                                                <label>
                                                    <span class="k">Duration</span>
                                                    <input
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        type="text"
                                                        placeholder="Instantaneous"
                                                        name="${base}-duration"
                                                        .value="${spell.duration}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="duration"
                                                        @input=${this.updateSpellField}
                                                    />
                                                </label>

                                                <label class="full">
                                                    <span class="k">Spell Text</span>
                                                    <textarea
                                                        @keydown=${this.noopEvent}
                                                        @keyup=${this.noopEvent}
                                                        rows="5"
                                                        placeholder="Describe the spell..."
                                                        name="${base}-text"
                                                        .value="${spell.text}"
                                                        data-level="${lvl}"
                                                        data-index="${index}"
                                                        data-field="text"
                                                        @input=${this.updateSpellField}
                                                    >${spell.text}</textarea>
                                                </label>
                                            </spell-grid>
                                        </spell-card>
                                    `;
                                })}
                            </spells-list>

                            <button
                                type="button"
                                class="bttn"
                                kind="dashed"
                                dull
                                color="warning"
                                data-level="${lvl}"
                                @click=${this.addSpell}
                            >
                                ${this.model.addLabel}
                            </button>
                        </spell-level>
                    `;
                })}
            </levels-grid>
        `;

        render(view, this);
    }
}

env.bind("spell-slots-table", SpellSlotsTable);
