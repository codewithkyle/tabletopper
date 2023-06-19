import env from "~brixi/controllers/env";
import sound from "~brixi/controllers/soundscape";

export default class PingComponent extends HTMLElement{
    constructor(x:number, y:number, color:string, zoom:number){
        super();
        if (zoom < 1){
            zoom = 2;
        }
        this.style.transform = `translate(${x}px, ${y}px) scale(${zoom})`;
        this.style.color = `var(--${color})`;
        this.className = "ping";
    }

    connectedCallback(){
        Promise.all([
            sound.add("ping", "/static/ping.mp3"),
            env.css(["ping"]),
        ])
        .then(() => {
            sound.setVolume("ping", 0.25);
            this.render();
        });
    }

    private render(){
        this.innerHTML = `
            <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                <path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"></path>
                <path d="M12 8l0 4"></path>
                <path d="M12 16l.01 0"></path>
            </svg>
        `;
        sound.play("ping");
        setTimeout(() => {
            this.remove();
        }, 1000);
    }
}
env.bind("ping-component", PingComponent);
