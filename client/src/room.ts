import { subscribe } from "@codewithkyle/pubsub";
import PlayerMenu from "~components/window/windows/player-menu/player-menu";
import Window from "~components/window/window";
import { connect, send } from "~controllers/ws";
import { Player } from "~types/app";
import DiceBox from "~components/window/windows/dice-box/dice-box";
import Initiative from "~components/window/windows/initiative/initiative";
import MonsterManual from "~components/window/windows/monster-manual/monster-manual";
import MonsterStatBlock from "~components/window/windows/monster-stat-block/monster-stat-block";

declare const htmx: any;

class Room {
    public uid: string;
    private room: string;
    private character: string;
    public isGM: boolean;
    private players: Player[];
    public gridSize: number;
    public activeInitiative: string|null;
    private initiative: Array<any>;

    constructor() {
        this.uid = "";
        this.room = "";
        this.character = "";
        this.isGM = false;
        this.players = [];
        this.gridSize = 32;
        this.initiative = [];
        this.activeInitiative = null;
        subscribe("socket", this.inbox.bind(this));
        this.init();
    }

    private async inbox({ type, data }){
        switch (type){
            case "room:initiative:sync":
                this.initiative = data;
                break;
            case "room:initiative:active":
                this.activeInitiative = data;
                break;
            case "room:tabletop:map:update":
                this.gridSize = data.cellSize;
                break;
            case "room:sync:players":
                this.players = data;
                break;
            case "core:init":
                const urlSegments = location.pathname.split("/");
                if (urlSegments.length < 3){
                    this.uid = localStorage.getItem("uid");
                    this.room = "";
                    this.character = "Game Master";
                    this.isGM = true;
                    send("room:create", this.uid);
                } else {
                    const roomCode = urlSegments[urlSegments.length - 1];
                    const room = JSON.parse(localStorage.getItem(`room:${roomCode.toUpperCase().trim()}`));
                    if (!room) location.href = location.origin;
                    this.isGM = false;
                    this.uid = room.uid;
                    this.room = room.room;
                    this.character = room.character;
                    send("room:join", {
                        id: this.uid,
                        name: this.character,
                        room: this.room,
                    });
                }
                break;
            case "room:create":
                fetch(`/session/gm/${data}`, {
                    method: "POST",
                });
                this.room = data;
                localStorage.setItem(`room:${data.toUpperCase().trim()}`, JSON.stringify({
                    uid: this.uid,
                    room: this.room,
                    character: this.character,
                }));
                window.history.replaceState({}, "", `/room/${data.toUpperCase().trim()}`);
                htmx.ajax("GET", "/stub/toolbar", "tool-bar");
                break;
            case "room:joined":
                if (data){
                    await fetch(`/session/gm/${data}`, {
                        method: "POST",
                    });
                    this.isGM = true;
                } else {
                    await fetch(`/session/gm`, {
                        method: "DELETE",
                    });
                }
                htmx.ajax("GET", "/stub/toolbar", "tool-bar");
                break;
            default:
                break;
        }
    }

    async init(){
        connect();
        window.addEventListener("room:lock", () => {
            send("room:lock");
        });
        window.addEventListener("room:unlock", () => {
            send("room:unlock");
        });
        window.addEventListener("room:players", () => {
            const window = document.body.querySelector('window-component[window="players"]') || new Window({
                name: "Players",
                view: new PlayerMenu(this.players),
            });
            if (!window.isConnected){
                document.body.append(window);
            }
        });
        window.addEventListener("tabletop:clear", () => {
            send("room:tabletop:map:clear");
        });
        window.addEventListener("tabletop:load", (e:CustomEvent) => {
            const { id } = e.detail;
            send("room:tabletop:map:load", id);
        });
        window.addEventListener("tabletop:update", (e:CustomEvent) => {
            const { cellSize, renderGrid } = e.detail;
            send("room:tabletop:map:update", { cellSize, renderGrid });
        });
        window.addEventListener("tabletop:spawn-pawns", () => {
            send("room:tabletop:spawn:players");
        });
        window.addEventListener("tabletop:spawn-monster", (e:CustomEvent) => {
            const { uid, name, hp, ac, size, image } = e.detail;
            const x = parseInt(sessionStorage.getItem("tabletop:spawn-monster:x"));
            const y = parseInt(sessionStorage.getItem("tabletop:spawn-monster:y"));
            send("room:tabletop:spawn:monster", {
                x: x,
                y: y,
                name: name, 
                hp: parseInt(hp), 
                ac: parseInt(ac), 
                size: size,
                image: image,
                monsterId: uid,
            });
        });
        window.addEventListener("window:dicebox", () => {
            const window = document.body.querySelector('window-component[window="dicebox"]') || new Window({
                name: "Dicebox",
                view: new DiceBox(),
                width: 300,
                height: 350,
            });
            if (!window.isConnected){
                document.body.append(window);
            }
        });
        window.addEventListener("window:initiative", () => {
            const w = document.body.querySelector('window-component[window="initiative"]') || new Window({
                name: "Initiative",
                view: new Initiative(this.initiative),
                width: 300,
                height: 350,
            });
            if (!w.isConnected){
                document.body.append(w);
            }
        });
        window.addEventListener("initiative:sync", () => {
            const pawns: HTMLElement[] = Array.from(document.body.querySelectorAll("pawn-component"));
            const claimedNames = [];
            if (this.initiative.length){
                for (let i = 0; i < this.initiative.length; i++){
                    claimedNames.push(this.initiative[i].name);
                }
            }
            for (let i = 0; i < pawns.length; i++){
                const pawnType = pawns[i].getAttribute("pawn");
                if (pawnType !== "dead" && !claimedNames.includes(pawns[i].dataset.name)){
                    claimedNames.push(pawns[i].dataset.name);
                    this.initiative.push({
                        uid: pawns[i].dataset.uid,
                        name: pawns[i].dataset.name,
                        type: pawnType,
                        index: i,
                    });
                }
            }
            send("room:initiative:sync", this.initiative);
            const w = document.body.querySelector('window-component[window="initiative"]') || new Window({
                name: "Initiative",
                view: new Initiative(this.initiative),
                width: 300,
                height: 350,
            });
            if (!w.isConnected){
                document.body.append(w);
            }
        });
        window.addEventListener("initiative:clear", () => {
            send("room:initiative:clear");
        });
        window.addEventListener("window:monsters", () => {
            const window = document.body.querySelector('window-component[window="monsters"]') || new Window({
                name: "Monsters",
                view: new MonsterManual(),
                width: 600,
                height: 650,
            });
            if (!window.isConnected){
                document.body.append(window);
            }
        });
        window.addEventListener("window:monsters:open", (e:CustomEvent) => {
            const { uid, name } = e.detail;
            const window = document.body.querySelector(`window-component[window="monster-${uid}"]`) || new Window({
                name: name,
                view: new MonsterStatBlock(uid),
                width: 400,
                height: 600,
            });
            if (!window.isConnected){
                document.body.append(window);
            }
        });
    }
}
const room = new Room();
export default room;
