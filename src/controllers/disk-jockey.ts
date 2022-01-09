class DJ {
    private music: {
        [key:string]: {
            el: HTMLAudioElement,
            playing: boolean,
            volume?: number,
        },
    };

    constructor(){
        this.music = {
            mainMenu: {
                el: new Audio("/music/177_Tavern_Music.mp3"),
                playing: false,
                volume: 0.15,
            }
        };
    }

    public play(key:string):void{
        if (key in this.music && !this.music[key].playing){
            this.music[key].playing = true;
            this.music[key].el.muted = false;
            this.music[key].el.volume = this.music[key]?.volume ?? 1;
            this.music[key].el.play();
        }
    }

    public pause(key:string):void{
        if (key in this.music && this.music[key].playing){
            this.music[key].el.volume = 0;
            this.music[key].el.muted = true;
            this.music[key].el.pause();
            this.music[key].playing = false;
        }
    }
}
const dj = new DJ();
export default dj;