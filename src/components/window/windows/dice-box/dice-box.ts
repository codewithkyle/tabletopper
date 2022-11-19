import SuperComponent from "@codewithkyle/supercomponent";
import {html, render} from "lit-html";
import env from "~brixi/controllers/env";
import db from "@codewithkyle/jsql";
import Button from "~brixi/components/buttons/button/button";
import notifications from "~brixi/controllers/notifications";
import {UUID} from "@codewithkyle/uuid";

// @ts-ignore
const DiceRoll: any = rpgDiceRoller.DiceRoll;

interface IDiceBox {}
export default class DiceBox extends SuperComponent<IDiceBox>{
    constructor(){
        super();
    }

    override async connected(){
        await env.css(["dice-box"]);
        this.render();
    }

    private async doRoll(){
        const input = this.querySelector("input") as HTMLInputElement;
        const roll = input.value.trim();
        if (roll.indexOf("*") !== -1 || roll.indexOf("/") !== -1){
            notifications.error("Dice Tray Error", "The dice tray does not support division or multiplication.");
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
        await db.query("INSERT INTO rolls VALUES ($roll)", {
            roll: {
                uid: UUID(),
                room: sessionStorage.getItem("room"),
                result: result,
                timestamp: new Date().getTime(),
            }
        });
        this.render();
    }

    private handleKeypress:EventListener = (e:Event) => {
        if (e instanceof KeyboardEvent){
            if (e.key.toLowerCase() === "enter"){
                this.doRoll();
            }
        }
    }

    override async render() {
        const log = await db.query("SELECT * FROM rolls WHERE room = $room ORDER BY timestamp", {
            room: sessionStorage.getItem("room"),
        });
        const view = html`
            <dice-log>
                ${log.map(roll => {
                    return html`
                        <p>${roll.result}</p>
                    `;
                })}
            </dice-log>
            <div class="w-full" flex="items-center row nowrap">
                <input type="text" placeholder="Dice codes" @keypress=${this.handleKeypress}>
                ${new Button({
                    label: "Roll",
                    kind: "solid",
                    color: "white",
                    class: "ml-0.5",
                    callback: this.doRoll.bind(this),
                })}
            </div>
            <p class="block font-xs font-grey-600 px-0.125">Example: 1d20 + 1d6 + 4</p>
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
