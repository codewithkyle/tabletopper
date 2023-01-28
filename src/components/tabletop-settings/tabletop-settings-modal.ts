import db from "@codewithkyle/jsql";
import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import NumberInput from "~brixi/components/inputs/number-input/number-input";
import env from "~brixi/controllers/env";
import cc from "controllers/control-center";
import Lightswitch from "~brixi/components/lightswitch/lightswitch";

interface ITabletopSettingsModal {
    gridSize: number,
    renderGrid: boolean,
}
export default class TabletopSettingsModal extends SuperComponent<ITabletopSettingsModal>{
    constructor(){
        super();
        this.model = {
            gridSize: 32,
            renderGrid: false,
        };
    }

    override async connected(){
        await env.css(["tabletop-settings-modal"]);
        const result = (await db.query("SELECT * FROM games WHERE uid = $room", { room: sessionStorage.getItem("room") }))[0];
        this.set({
            gridSize: result["grid_size"],
            renderGrid: result["render_grid"],
        });
    }

    private handleBackdropClick:EventListener = (e:Event) => {
        this.remove();
    }

    private handleGridSizeInput(value){
        this.set({
            gridSize: parseInt(value),
        }, true);
    }

    private saveSettings(){
        const op = cc.set("games", sessionStorage.getItem("room"), "grid_size", this.model.gridSize);
        const op2 = cc.set("games", sessionStorage.getItem("room"), "render_grid", this.model.renderGrid);
        cc.dispatch(cc.batch("games", sessionStorage.getItem("room"), [op, op2]));
        this.remove();
    }

    override render(){
        const view = html`
            <div class="backdrop" @click=${this.handleBackdropClick}></div>
            <div class="modal">
                <div class="w-full p-2" grid="columns 1 gap-1.5">
                    ${new NumberInput({
                        value: this.model.gridSize,
                        label: "Grid Cell Size",
                        instructions: "Units in pixels.",
                        callback: this.handleGridSizeInput.bind(this),
                        name: "grid-cell-size",
                        icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="4" y1="7" x2="20" y2="7"></line><line x1="4" y1="17" x2="20" y2="17"></line><line x1="7" y1="4" x2="7" y2="20"></line><line x1="17" y1="4" x2="17" y2="20"></line></svg>`,
                    })}
                    ${new Lightswitch({
                        label: "Grid hidden",
                        altLabel: "Grid visible",
                        enabled: this.model.renderGrid,
                        callback: (enabled:boolean)=>{
                            this.set({
                                renderGrid: enabled,
                            }, true);
                        }
                    })}
                </div>
                <div class="w-full p-1 bg-grey-50 border-t-1 border-t-solid border-t-grey-300" flex="row nowrap items-center justify-end">
                    ${new Button({
                        label: "Cancel",
                        kind: "solid",
                        color: "white",
                        class: "mr-1",
                        callback: ()=>{
                            this.remove();
                        }
                    })}
                    ${new Button({
                        label: "Save",
                        kind: "solid",
                        color: "success",
                        callback: this.saveSettings.bind(this),
                    })}
                </div>
            </div>
        `;
        render(view, this);
    }
}
env.bind("tabletop-settings-modal", TabletopSettingsModal);
