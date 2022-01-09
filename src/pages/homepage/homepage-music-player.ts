import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import Button from "~brixi/components/buttons/button/button";
import env from "~brixi/controllers/env";
import dj from "~controllers/disk-jockey";

interface IHomepageMusicPlayer {
    playing: boolean;
}
export default class HomepageMusicPlayer extends SuperComponent<IHomepageMusicPlayer>{

    constructor(){
        super();
        this.model = {
            playing: true,
        };
    }

    override async connected() {
       await env.css(["homepage-music-player"]);
       dj.play("mainMenu");
       this.render();
    }

    private togglePlayback():void{
        if (this.model.playing){
            this.set({
                playing: false,
            });
            dj.pause("mainMenu");
        } else {
            this.set({
                playing: true,
            });
            dj.play("mainMenu");
        }
    }

    private renderIcon():string{
        let out;
        if (this.model.playing){
            out = `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-volume" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M15 8a5 5 0 0 1 0 8"></path><path d="M17.7 5a9 9 0 0 1 0 14"></path><path d="M6 15h-2a1 1 0 0 1 -1 -1v-4a1 1 0 0 1 1 -1h2l3.5 -4.5a0.8 .8 0 0 1 1.5 .5v14a0.8 .8 0 0 1 -1.5 .5l-3.5 -4.5"></path></svg>`;
        } else {
            out = `<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-volume-3" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M6 15h-2a1 1 0 0 1 -1 -1v-4a1 1 0 0 1 1 -1h2l3.5 -4.5a0.8 .8 0 0 1 1.5 .5v14a0.8 .8 0 0 1 -1.5 .5l-3.5 -4.5"></path><path d="M16 10l4 4m0 -4l-4 4"></path></svg>`;
        }
        return out;
    }

    override render(): void {
        const view = html`
            ${new Button({
                kind: "text",
                color: "white",
                icon: this.renderIcon(),
                iconPosition: "center",
                callback: this.togglePlayback.bind(this),
                tooltip: `${this.model.playing ? "Disabled" : "Enable"} music`
            })}
        `;
        render(view, this);
    }
}
env.bind("homepage-music-player", HomepageMusicPlayer);