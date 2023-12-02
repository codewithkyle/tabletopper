import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import type { Monster } from "~types/app";

declare const htmx: any;

interface IMonsterStatBlock extends Monster {};
export default class MonsterStatBlock extends SuperComponent<IMonsterStatBlock>{
    private uid:string;

    constructor(uid:string){
        super();
        this.uid = uid;
    }

    override async connected(){
        await env.css(["monster-stat-block"]);
        htmx.ajax("GET", `/stub/windows/monsters/${this.uid}`, { target: this });
    }
}
env.bind("monster-stat-block", MonsterStatBlock);
