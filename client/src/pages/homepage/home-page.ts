import { subscribe } from "@codewithkyle/pubsub";
import { navigateTo } from "@codewithkyle/router";
import SuperComponent from "@codewithkyle/supercomponent";
import { render, html, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import Input from "~brixi/components/inputs/input/input";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import cc from "~controllers/control-center";
import { connected, send } from "~controllers/ws";
import { randomInt } from "../../utils/math";
import HomepageMusicPlayer from "./homepage-music-player/homepage-music-player";
import PlayerTokenPicker from "./player-token-picker/player-token-picker";
import sound from "~brixi/controllers/soundscape";
import alerts from "~brixi/controllers/alerts";

const COLORS = ["grey", "red", "orange", "amber", "yellow", "lime", "green", "emerald", "teal", "cyan", "light-blue", "blue", "indigo", "violet", "purple", "fuchsia", "pink", "rose"];

interface IHomepage{
    connected: boolean,
    code: string,
}
export default class Homepage extends SuperComponent<IHomepage>{
    constructor(){
        super();
        this.state = "WELCOME";
        this.stateMachine = {
            WELCOME: {
                NEXT: "MENU",
            },
            MENU: {
                JOIN: "ROOM"
            },
            ROOM: {
                NEXT: "CHARACTER",
                BACK: "MENU",
            },
            CHARACTER: {
                BACK: "ROOM",
            },
        };
        this.model = {
            connected: connected,
            code: "",
        };
        if (sessionStorage.getItem("kicked")){
            alerts.error("Kicked From Game", "You have been kicked by the Game Master.");
            sessionStorage.removeItem("kicked");
        }
        subscribe("socket", this.inbox.bind(this));
    }

    private inbox({ type, data}){
        switch(type){
            case "connected":
                if (!this.model.connected){
                    this.set({
                        connected: true,
                    });
                }
                break;
            case "disconnected":
                if (this.model.connected){
                    this.set({
                        connected: false,
                    });
                }
                break;
            case "room:join":
                console.log("pausing main menu music");
                sound.pause("mainMenu");
                navigateTo(`/room/${data.uid}`);
                break;
            case "character:getDetails":
                this.trigger("NEXT");
                break;
            default:
                break;
        }
    }

    override async connected() {
        await env.css(["homepage"]);
        this.render();
    }

    private submitRoom:EventListener = (e:Event) => {
        e.preventDefault();
        const input = this.querySelector("[form-input]") as Input;
        const value = input.getValue().toString().trim().toUpperCase();
        if (input.validate()){
            const lastConfirmedCode = sessionStorage.getItem("room");
            const lastConfiredSocket = sessionStorage.getItem("lastSocketId");
            if (lastConfirmedCode === value && lastConfiredSocket != null){
                navigateTo(`/room/${value}`);
            } else {
                send("room:check", {
                    code: value,
                });
            }
        }
    }

    private submitCharacter:EventListener = async (e:Event) => {
        e.preventDefault();
        const input = this.querySelector("[form-input]") as Input;
        const tokenPicker = this.querySelector("player-token-picker") as PlayerTokenPicker;
        if (input.validate()){
            const image = await tokenPicker.getValue();
            let imageOP = null;
            if (image){
                imageOP = cc.insert("images", image.uid, image);
            }
            const name = input.getValue().toString();
            const color = `${COLORS[randomInt(0, COLORS.length)]}-500`;
            sessionStorage.setItem("color", color);
            const player = cc.insert("players", sessionStorage.getItem("socketId"), {
                uid: sessionStorage.getItem("socketId"),
                name: name,
                token: image?.uid ?? null,
                active: true,
                color: color,
            });
            send("room:join", {
                name: name,
                player: player,
                token: imageOP,
            });
            sessionStorage.setItem("role", "player");
        }
    }

    private renderActions(): TemplateResult {
       return html`
            <div class="actions w-full" flex="row nowrap items-center justify-center">
                ${new Button({
                    callback: ()=>{
                        this.trigger("JOIN");
                    },
                    label: "Join Room",
                    kind: "outline",
                    color: "white",
                    class: "mr-1",
                    size: "large",
                    icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-login" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M14 8v-2a2 2 0 0 0 -2 -2h-7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h7a2 2 0 0 0 2 -2v-2"></path><path d="M20 12h-13l3 -3m0 6l-3 -3"></path></svg>`, 
                })}
                ${new Button({
                        callback: ()=>{
                            send("create:room");
                            sessionStorage.setItem("role", "gm");
                            const color = `${COLORS[randomInt(0, COLORS.length)]}-500`;
                            sessionStorage.setItem("color", color);
                        },
                        label: "Create Room",
                        kind: "outline",
                        color: "white",
                        size: "large",
                        icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-circle-plus" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><circle cx="12" cy="12" r="9"></circle><line x1="9" y1="12" x2="15" y2="12"></line><line x1="12" y1="9" x2="12" y2="15"></line></svg>`,
                    })}
                </div>
            </div> 
       `;
   }

   private renderLoading():TemplateResult{
       return html`
            <div flex="items-center justify-center row nowrap" class="actions w-full" style="height:42px;">
                ${new Spinner({
                    size: 24,
                    color: "white"
                })}
                <span class="ml-0.5 font-lg font-medium font-white">Connecting to online services...</span>
            </div>
       `;
   }

   private renderWelcome():TemplateResult{
       return html`
            
       `;
   }

   private renderMenu():TemplateResult{
       return html`
            <div class="w-1024 menu text-center">
                <h1>Tabletopper</h1>
                ${this.model.connected ? this.renderActions() : this.renderLoading()}
            </div>
            ${new HomepageMusicPlayer()}
       `;
   }

   private renderJoin():TemplateResult{
       return html`
            
            ${new HomepageMusicPlayer()}
       `;
   }

   private renderCharacter():TemplateResult{
       return html`
            <form @submit=${this.submitCharacter} class="join w-411 bg-white border-1 border-solid border-grey-300 shadow-grey-sm radius-0.5 no-scroll">
                <div class="p-1.5" flex="row nowrap items-center">
                    ${new PlayerTokenPicker()}
                    ${new Input({
                        class: "ml-1.5 mb-0.25",
                        name: "name",
                        label: "Character Name",
                        required: true,
                        css: "flex:1;",
                    })}
                </div>
                <div flex="row nowrap items-center justify-end" class="p-1 bg-grey-100 border-t-1 border-t-solid border-t-grey-300">
                    ${new Button({
                        type: "button",
                        label: "Back",
                        class: "mr-1",
                        callback: ()=> {
                            this.trigger("BACK");
                        },
                    })}
                    <button type="submit" class="bttn" color="primary" kind="solid">Join Room</button>
                </div>
            </form>
            ${new HomepageMusicPlayer()}
       `;
   }
}
