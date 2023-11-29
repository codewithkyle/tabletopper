import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";

declare const htmx: any;

interface IMonsterManual {}
export default class MonsterManual extends SuperComponent<IMonsterManual>{
    override async connected(){
        await env.css(["monster-manual"]);
        htmx.ajax("GET", "/stub/windows/monsters", { target: this });
    }
}
env.bind("monster-manual", MonsterManual);
