import env from "~brixi/controllers/env";

export default class PingComponent extends HTMLElement{
    private audio:HTMLAudioElement;

    constructor(x:number, y:number, color:string, zoom:number){
        super();
        if (zoom < 1){
            zoom = 2;
        }
        this.style.transform = `translate(${x}px, ${y}px) scale(${zoom})`;
        this.style.color = `var(--${color})`;
        this.className = "ping";
        this.audio = new Audio("/static/ping.mp3");
    }

    connectedCallback(){
        env.css(["ping"]).then(() => {
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
        this.audio.volume = 0.25;
        this.audio.play();
        setTimeout(() => {
            this.remove();
        }, 1000);
    }
}
env.bind("ping-component", PingComponent);
