class DJ {
    public music: {
        [key:string]: {
            el: HTMLAudioElement,
            playing: boolean,
            muted: boolean,
            volume?: number,
        },
    };

    constructor(){
        this.music = {
            mainMenu: {
                el: new Audio("/music/177_Tavern_Music.mp3"),
                playing: false,
                muted: false,
                volume: 0.15,
            }
        };
    }

    public mute(key:string):void{
        if (key in this.music){
            this.music[key].muted = true;
            this.music[key].el.muted = true;
        }
    }

    public unmute(key:string):void{
        if (key in this.music){
            this.music[key].muted = false;
            this.music[key].el.muted = false;
        }
    }

    public play(key:string):void{
        if (key in this.music && !this.music[key].playing){
            this.music[key].playing = true;
            this.music[key].el.muted = this.music[key].muted;
            this.music[key].el.volume = this.music[key]?.volume ?? 1;
            this.music[key].el.play();
        }
    }

    public pause(key:string):void{
        if (key in this.music && this.music[key].playing){
            this.music[key].el.volume = 0;
            this.music[key].el.pause();
            this.music[key].playing = false;
        }
    }
}
const dj = new DJ();
export default dj;