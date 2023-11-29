import SuperComponent from "@codewithkyle/supercomponent";
import { html, render } from "lit-html";
import { unsafeHTML } from "lit-html/directives/unsafe-html";
import env from "~brixi/controllers/env";
import { parseDataset } from "~brixi/utils/general";
import { Ability } from "~types/app";

interface IMonsterInfoTable {
    rows: Ability[],
    name: string,
    label: string,
    addLabel: string,
}
class MonsterInfoTable extends SuperComponent<IMonsterInfoTable>{

    constructor(){
        super();
        this.model = {
            label: "",
            rows: [],
            name: "",
            addLabel: "Add Row",
        };
    }

    static get observedAttributes() {
        return [
            "data-label",
            "data-name",
            "data-add-label",
        ];
    }

    override async connected(){
        await env.css(["monster-info-table", "buttons"]);
        const settings = parseDataset(this.dataset, this.model);
        this.set(settings);
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
        this.setAttribute("form-input", "");
        const view = html`
            <h4 class="block w-full font-medium font-sm font-grey-800 dark:font-grey-300 pl-0.125">${unsafeHTML(this.model.label)}</h4>
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
            <button type="button" class="bttn" kind="dashed" dull color="warning" @click=${this.addRow}>${this.model.addLabel}</button>
        `;
        render(view, this);
    }
}
env.bind("monster-info-table", MonsterInfoTable);
