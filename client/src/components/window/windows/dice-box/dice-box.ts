import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import alerts from "~brixi/controllers/alerts";

// @ts-ignore
const DiceRoll: any = rpgDiceRoller.DiceRoll;

interface IDiceBox {}
export default class DiceBox extends SuperComponent<IDiceBox>{
    private log: Array<string>;

    constructor(){
        super();
        this.log = [];
    }

    override async connected(){
        await env.css(["dice-box"]);
        this.render();
    }

    private async doRoll(){
        const input = this.querySelector("input") as HTMLInputElement;
        const roll = input.value.trim().toLowerCase();
        if (roll.indexOf("*") !== -1 || roll.indexOf("/") !== -1){
            alerts.error("Dice Tray Error", "The dice tray does not support division or multiplication.", [], 10);
            return;
        }
        const results = new DiceRoll(roll);
        const rolls = [];
        let hardAdd = 0;
        for (let i = 0; i < results.rolls.length; i++){
            const slot = results.rolls[i];
            if (slot?.rolls){
                for (let j = 0; j < slot.rolls.length; j++){
                    rolls.push(slot.rolls[j].value);
                }
            } else if (typeof slot === "number"){
                switch (results.rolls[i - 1]){
                    case "+":
                        hardAdd += slot;
                        break;
                    case "-":
                        hardAdd -= slot;
                        break;
                    default:
                        break;
                }
            }
        }
        let result = `[${rolls.join(", ")}]`;
        if (hardAdd !== 0){
            if (hardAdd > 0){
                result += ` + ${hardAdd}`;
            } else {
                result += ` - ${hardAdd}`;
            }
        }
        result += ` = ${results.total}`;
        input.value = "";
        this.log.push(result);
        this.render();
    }

    private handleKeypress:EventListener = (e:Event) => {
        e.stopImmediatePropagation()
        if (e instanceof KeyboardEvent){
            if (e.key.toLowerCase() === "enter"){
                this.doRoll();
            }
        }
    }

    private noopEvent:EventListener = (e:Event) => {
        e.stopImmediatePropagation();
    }

    override async render() {
        const view = html`
            <dice-log>
                ${this.log.map(roll => {
                    return html`
                        <p>${roll}</p>
                    `;
                })}
            </dice-log>
            <div class="w-full" flex="items-center row nowrap">
                <input autofocus type="text" placeholder="Dice codes" @keypress=${this.handleKeypress}  @keydown=${this.noopEvent} @keyup=${this.noopEvent}>
            </div>
            <p class="block font-xs font-grey-600 dark:font-grey-400 px-0.125">Example: 1d20 + 1d6 + 4</p>
        `;
        render(view, this);
        const diceLog = this.querySelector("dice-log");
        diceLog.scrollTo({
            top: diceLog.scrollHeight,
            left: 0,
        })
    }
}
env.bind("dice-box", DiceBox);
