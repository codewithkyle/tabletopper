import { subscribe } from "@codewithkyle/pubsub";
import PlayerMenu from "~components/window/windows/player-menu/player-menu";
import Window from "~components/window/window";
import { connect, send } from "~controllers/ws";
import { Player } from "~types/app";
import DiceBox from "~components/window/windows/dice-box/dice-box";

declare const htmx: any;

class Room {
    public uid: string;
    private room: string;
    private character: string;
    public isGM: boolean;
    private players: Player[];
    public gridSize: number;

    constructor() {
        this.uid = "";
        this.room = "";
        this.character = "";
        this.isGM = false;
        this.players = [];
        this.gridSize = 32;
        subscribe("socket", this.inbox.bind(this));
        this.init();
    }

    private async inbox({ type, data }){
        switch (type){
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
        window.addEventListener("window:dicebox", () => {
            const window = document.body.querySelector('window-component[window="dicebox"]') || new Window({
                name: "Dicebox",
                view: new DiceBox(),
                width: 200,
                height: 350,
            });
            if (!window.isConnected){
                document.body.append(window);
            }
        });
    }
}
const room = new Room();
export default room;
