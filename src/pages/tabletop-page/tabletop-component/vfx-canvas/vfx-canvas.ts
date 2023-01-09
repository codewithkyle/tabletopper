import SuperComponent from "@codewithkyle/supercomponent";
import env from "~brixi/controllers/env";
import Pawn from "components/pawn/pawn";

interface IVFXCanvas {}
export default class VFXCanvas extends SuperComponent<IVFXCanvas>{
    private canvas:HTMLCanvasElement;
    private ctx:CanvasRenderingContext2D;
    private time:number;
    private effects:Array<BloodSpatter>;
    private images:Array<HTMLImageElement>;
    private img:HTMLImageElement;
    private running: boolean;

    constructor(img:HTMLImageElement){
        super();
        this.canvas = null;
        this.img = img;
        this.images = [];
        this.running = false;
        this.effects = [];
        for (let i = 1; i <= 5; i++){
            const image = new Image();
            image.src = `${location.origin}/images/blood-${i}.png`;
            image.style.objectFit = "contain";
            image.onload = () => {
                this.images.push(image);
            }
        }
    }

    override async connected(){
        await env.css(["vfx-canvas"]);
        this.render();
    }

    private cleanup(){
        for (let i = this.effects.length - 1; i >= 0; i--){
            if (this.effects[i].dead){
                this.effects.splice(i, 1);
            }
        }
    }

    private randomInt(min, max){
        return Math.floor(Math.random() * (max - min + 1) + min);
    }

    private renderLoop(){
        this.running = true;
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        const newTime = performance.now();
        const deltaTime = (newTime - this.time) / 1000;
        this.time = newTime;

        const bleedingPawns:Array<Pawn> = Array.from(document.body.querySelectorAll(`pawn-component[bleeding="true"][ghost="false"]`));
        if (this.images.length){
            for (let i = 0; i < bleedingPawns.length; i++){
                bleedingPawns[i].timeToSplatter -= deltaTime;
                if (bleedingPawns[i].timeToSplatter <= 0){
                    bleedingPawns[i].timeToSplatter = this.randomInt(0, 2);
                    const imageIndex = this.randomInt(0, this.images.length - 1);
                    const pawnX = parseInt(bleedingPawns[i].dataset.x);
                    const pawnY = parseInt(bleedingPawns[i].dataset.y);
                    const pawnW = parseInt(bleedingPawns[i].dataset.w);
                    const pawnH = parseInt(bleedingPawns[i].dataset.h);
                    const centerX = this.canvas.width * 0.5;
                    const centerY = this.canvas.height * 0.5;
                    const x = Math.round(centerX + pawnX);
                    const y = Math.round(centerY + pawnY)
                    const pos = [
                        this.randomInt((x - (pawnW * 0.125)), (x + (pawnW * 0.125))),
                        this.randomInt((y - (pawnH * 0.125)), (y + (pawnH * 0.125)))
                    ];
                    const splatter = new BloodSpatter(this.images[imageIndex], pos, pawnW / this.parentElement.zoom);
                    this.effects.push(splatter);
                }
            }
        }

        for (let i = 0; i < this.effects.length; i++){
            this.effects[i].render(this.ctx, deltaTime);
        }

        this.cleanup();
        window.requestAnimationFrame(this.renderLoop.bind(this));
    }

    override render(): void {
        if (!this.canvas){
            this.canvas = document.createElement("canvas");
            this.appendChild(this.canvas);
        }
        this.canvas.width = this.img.width;
        this.canvas.height = this.img.height;
        this.canvas.style.width = `${this.img.width}px`;
        this.canvas.style.height = `${this.img.height}px`;
        this.ctx = this.canvas.getContext("2d");
        if (!this.running){
            this.time = performance.now();
            this.renderLoop();
        }
    }
}
env.bind("vfx-canvas", VFXCanvas);

class BloodSpatter{
    public dead:boolean;
    private image:HTMLImageElement;
    private size: number;
    private position:Array<number>;
    private life:number;

    constructor(image:HTMLImageElement, position:Array<number>, size:number){
        this.image = image;
        this.position = position;
        this.size = size;
        this.life = 3;
        this.image.width = size;
        this.image.height = size;
    }

    public render(ctx:CanvasRenderingContext2D, deltaTime:number){
        ctx.globalAlpha = this.life;
        ctx.drawImage(this.image, this.position[0], this.position[1], this.size, this.size);
        ctx.globalAlpha = 1;
        this.life -= deltaTime;
        if (this.life <= 0){
            this.dead = true;
        }
    }
}
