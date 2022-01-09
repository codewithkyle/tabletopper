import { subscribe } from "@codewithkyle/pubsub";
import SuperComponent from "@codewithkyle/supercomponent";
import { render, html, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import Spinner from "~brixi/components/progress/spinner/spinner";
import env from "~brixi/controllers/env";
import { connect } from "~controllers/ws";
import HomepageMusicPlayer from "./homepage-music-player";

interface IHomepage{
    connected: boolean,
}
export default class Homepage extends SuperComponent<IHomepage>{
    constructor(){
        super();
        this.state = "WELCOME";
        this.stateMachine = {
            WELCOME: {
                NEXT: "MENU",
            }
        }
        this.model = {
            connected: false,
        };
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
            default:
                break;
        }
    }

    override async connected() {
        await env.css(["homepage"]);
        this.render();
        connect();
    }

   private renderActions(): TemplateResult {
       return html`
            <div class="actions w-full" flex="row nowrap items-center justify-center">
                ${new Button({
                    callback: ()=>{},
                    label: "Join Room",
                    kind: "outline",
                    color: "white",
                    class: "mr-1",
                    size: "large",
                    icon: `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-login" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M14 8v-2a2 2 0 0 0 -2 -2h-7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h7a2 2 0 0 0 2 -2v-2"></path><path d="M20 12h-13l3 -3m0 6l-3 -3"></path></svg>`, 
                })}
                ${new Button({
                        callback: ()=>{},
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
            <div class="w-768 bg-white border-1 border-solid border-grey-200 shadow-grey-sm radius-0.5 no-scroll">
                <h1 class="px-2 pt-1.75 font-grey-900 font-2xl line-normal font-bold block">Greetings Adventurer!</h1>
                <p class="px-2 pb-2 pt-1 font-grey-700 line-normal block">
                    Welcome to the alpha build of Tabletopper, a free web-based virtual tabletop (VTT). Before you continue we must warn you that this is an alpha build which means you may experience some bugs or glitches. Please report any issues through the help menu.
                    <br>
                    <br>
                    May your adventures be grand and your rewards bountiful.
                </p>
                <div flex="justify-end items-center row nowrap" class="p-1 bg-grey-50 border-t-1 border-t-solid border-t-grey-200">
                    ${new Button({
                        kind: "solid",
                        color: "info",
                        callback: ()=>{
                            this.trigger("NEXT");
                        },
                        label: "I understand",
                    })}
                </div>
            </div>
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

    override render(){
        let content;
        switch(this.state){
            case "WELCOME":
                content = this.renderWelcome();
                break;
            case "MENU":
                content = this.renderMenu();
                break;
        }
        const view = html`
            <img src="/images/background.jpg" width="1920" loading="lazy" draggable="false">
            <div class="w-full h-full" flex="justify-center items-center column wrap">
                ${content}
            </div>
        `;
        setTimeout(()=>{
            render(view, this);
        }, 100);
    }
}