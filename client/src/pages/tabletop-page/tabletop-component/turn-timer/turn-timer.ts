import { publish, subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import room from "room";
import { html, render } from "lit-html";
import { send } from "~controllers/ws";

interface ITurnTimerComponent {
    hidden: boolean;
    running: boolean;
}
export default class TurnTimerComponent extends SuperComponent<ITurnTimerComponent>{
    private loop: Function;
    private time: number;
    private timer: number;
    private hover: boolean;

    constructor(){
        super();
        this.model = {
            hidden: true,
            running: false,
        };
        this.loop = ()=>{};
        this.time = 0;
        this.timer = 0;
        this.hover = false;
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data }) {
        switch(type){
            case "room:initiative:turn:start":
                if (data === room.uid){
                    this.set({
                        hidden: false,
                        running: true,
                    });
                } else {
                    this.set({
                        hidden: true,
                        running: false,
                    });
                }
                break;
            default:
                break;
        }
    }

    private onEnter:EventListener = (e) => {
        this.hover = true;
    }
    private onLeave:EventListener = (e) => {
        this.hover = false;
    }
    private onClick:EventListener = (e) => {
        e.stopImmediatePropagation();
        send("room:initiative:next");
        this.set({
            hidden: true,
            running: false,
        });
    }

    override async connected() {
        await env.css(["turn-timer"]);
        this.addEventListener("mouseenter", this.onEnter);
        this.addEventListener("mouseleave", this.onLeave);
        this.addEventListener("focus", this.onEnter);
        this.addEventListener("blur", this.onLeave);
        this.addEventListener("click", this.onClick);
        this.render();
    }

    private _loop() {
        const newTime = performance.now();
        const deltaTime = (newTime - this.time) / 1000;
        this.time = newTime;

        this.timer += deltaTime;

        const minutes = Math.floor(this.timer / 60);
        const seconds = Math.floor(this.timer % 60);

        let view;
        if (this.hover){
            view = html`
                <span class="font-2xl">End Turn</span>
            `;
        } else {
            view = html`
                <span>${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}</span>
            `;
        }
        render(view, this);

        if (minutes === 0 && seconds >= 30) {
            this.setAttribute("color", "safe");
        } else if (minutes === 1) {
            this.setAttribute("color", "warning");
        } else if (minutes > 1) {
            this.setAttribute("color", "danger");
        }

        window.requestAnimationFrame(this.loop.bind(this));
    }

    override render(){
        if (this.model.hidden) {
            this.setAttribute("hidden", `${this.model.hidden}`);
            this.loop = ()=>{};
            return;
        }
        this.setAttribute("role", "button");
        this.setAttribute("tabindex", "0");
        this.removeAttribute("hidden");
        if (this.model.running){
            this.time = performance.now();
            this.timer = 0;
            this.loop = this._loop;
            this.loop();
        }
    }
}
env.bind("turn-timer", TurnTimerComponent);
