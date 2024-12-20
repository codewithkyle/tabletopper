import { subscribe } from "@codewithkyle/pubsub";
import PlayerMenu from "~components/window/windows/player-menu/player-menu";
import Window from "~components/window/window";
import { connect, send } from "~controllers/ws";
import { Player, Size } from "~types/app";
import DiceBox from "~components/window/windows/dice-box/dice-box";
import MonsterManual from "~components/window/windows/monster-manual/monster-manual";
import MonsterStatBlock from "~components/window/windows/monster-stat-block/monster-stat-block";
import FogBrush from "~components/window/windows/fog-brush/fog-brush";
import DoodleBrush from "~components/window/windows/doodle-brush/doodle-brush";
import TabeltopComponent from "pages/tabletop-page/tabletop-component/tabletop-component";

declare const htmx: any;

class Room {
    public uid: string;
    private room: string;
    private character: string;
    public isGM: boolean;
    private players: Player[];
    public gridSize: number;
    public cellDistance: number;
    public renderGrid: boolean;
    public prefillFog: boolean;
    public dmgOverlay: boolean;
    public activeInitiative: string | null;
    private hasLogOn: boolean;

    constructor() {
        this.uid = "";
        this.room = "";
        this.character = "";
        this.isGM = false;
        this.players = [];
        this.gridSize = 32;
        this.cellDistance = 5;
        this.renderGrid = false;
        this.prefillFog = false;
        this.dmgOverlay = false;
        this.activeInitiative = null;
        this.hasLogOn = false;
        subscribe("socket", this.inbox.bind(this));
        this.init();
    }

    private async inbox({ type, data }) {
        switch (type) {
            case "room:tabletop:map:update":
                this.gridSize = data.cellSize;
                this.renderGrid = data.renderGrid;
                this.cellDistance = data.cellDistance;
                this.prefillFog = data.prefillFog;
                this.dmgOverlay = data.dmgOverlay;
                break;
            case "room:sync:players":
                this.players = data;
                break;
            case "core:init":
                const urlSegments = location.pathname.split("/");
                if (urlSegments.length < 3) {
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
                        maxHP: room.maxHP,
                        hp: room.hp,
                        ac: room.ac,
                    });
                }
                break;
            case "room:create":
                this.room = data;
                localStorage.setItem(`room:${data.toUpperCase().trim()}`, JSON.stringify({
                    uid: this.uid,
                    room: this.room,
                    character: this.character,
                }));
                window.history.replaceState({}, "", `/room/${data.toUpperCase().trim()}`);
                htmx.ajax("GET", `/stub/toolbar?gm=1`, "tool-bar");
                break;
            case "room:joined":
                if (data === this.room) {
                    this.isGM = true;
                }
                send("room:player:rename", {
                    playerId: this.uid,
                    name: this.character,
                });
                htmx.ajax("GET", `/stub/toolbar?gm=${this.isGM ? '1' : '0'}`, "tool-bar");
                if (!this.isGM) {
                    const verifyReq = await fetch('/user/verify')
                    if (verifyReq.status === 200) {
                        this.hasLogOn = true;
                    }
                    if (this.hasLogOn) {
                        window.dispatchEvent(new CustomEvent("show-user-menu"));
                        window.addEventListener("character:load", this.onCharacterLoad);
                        window.addEventListener("character:cancel", this.onCharacterLoadCancel);
                    }
                }
                break;
            default:
                break;
        }
    }

    private onCharacterLoad:EventListener = (e:CustomEvent) => {
        const { id } = e.detail;
        send("room:player:image", id);
        window.removeEventListener("character:load", this.onCharacterLoad);
        window.removeEventListener("character:cancel", this.onCharacterLoadCancel);
    }
    private onCharacterLoadCancel:EventListener = () => {
        window.removeEventListener("character:load", this.onCharacterLoad);
        window.removeEventListener("character:cancel", this.onCharacterLoadCancel);
    }

    async init() {
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
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("tabletop:clear", () => {
            send("room:tabletop:map:clear");
        });
        window.addEventListener("tabletop:load", (e: CustomEvent) => {
            const { id } = e.detail;
            send("room:tabletop:map:load", id);
        });
        window.addEventListener("tabletop:update", (e: CustomEvent) => {
            const { cellSize, renderGrid, cellDistance, prefillFog, dmgOverlay } = e.detail;
            send("room:tabletop:map:update", { cellSize: parseInt(cellSize), renderGrid, cellDistance: parseInt(cellDistance), prefillFog, dmgOverlay });
        });
        window.addEventListener("tabletop:spawn-pawns", () => {
            send("room:tabletop:spawn:players");
        });
        window.addEventListener("tabletop:spawn-monster", (e: CustomEvent) => {
            const { uid, name, hp, ac, size, image, x, y, hidden } = e.detail;
            const tabletop = document.body.querySelector("tabletop-component") as TabeltopComponent;
            let diffX = (x - tabletop.x) / tabletop.zoom;
            let diffY = (y - tabletop.y) / tabletop.zoom;
            let multi = 1;
            switch (size as Size){
                case "tiny":
                    multi = 0.25;
                    break;
                case "small":
                    multi = 0.5;
                    break;
                case "large":
                    multi = 2;
                    break;
                case "huge":
                    multi = 3;
                    break;
                case "gargantuan":
                    multi = 4;
                    break;
                default:
                    multi = 1;
                    break;
            }
            const halfSize = this.gridSize * multi / 2;
            send("room:tabletop:spawn:monster", {
                x: Math.round(diffX) - halfSize,
                y: Math.round(diffY) - halfSize,
                name: name,
                hp: hp,
                ac: ac,
                size: size,
                image: image,
                monsterId: uid,
                hidden: hidden,
            });
        });
        window.addEventListener("tabletop:quick-spawn", () => {
            const inputs = document.body.querySelectorAll("quick-spawn [form-input]");
            const data = {};
            inputs.forEach((input) => {
                // @ts-ignore
                data[input.getName()] = input.getValue();
            });

            const x = parseInt(sessionStorage.getItem("tabletop:spawn-monster:x"));
            const y = parseInt(sessionStorage.getItem("tabletop:spawn-monster:y"));
            data["x"] = x;
            data["y"] = y;

            data["image"] = null;
            const imgInput = document.body.querySelector('quick-spawn input[name="image"]') as HTMLInputElement;
            if (imgInput && imgInput.value) {
                data["image"] = imgInput.value;
            }

            if (data["type"] === "npc") {
                data["ownerId"] = this.uid;
                send("room:tabletop:spawn:npc", data);
            } else {
                send("room:tabletop:spawn:monster", data);
            }
        });
        window.addEventListener("window:dicebox", () => {
            const window = document.body.querySelector('window-component[window="dicebox"]') || new Window({
                name: "Dicebox",
                view: new DiceBox(),
                width: 300,
                height: 350,
            });
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("window:monsters", () => {
            const window = document.body.querySelector('window-component[window="monsters"]') || new Window({
                name: "Monsters",
                view: new MonsterManual(),
                width: 600,
                height: 650,
            });
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("window:monsters:open", (e: CustomEvent) => {
            const { uid, name } = e.detail;
            const window = document.body.querySelector(`window-component[window="monster-${uid}"]`) || new Window({
                name: name,
                view: new MonsterStatBlock(uid),
                width: 400,
                height: 600,
            });
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("fog:fill", () => {
            send("room:tabletop:fog:fill");
        });
        window.addEventListener("fog:clear", () => {
            send("room:tabletop:fog:clear");
        });
        window.addEventListener("window:fog", () => {
            const name = "Fog of War Settings";
            const window = document.body.querySelector(`window-component[window="fog-of-war-settings"]`) || new Window({
                name: name,
                view: new FogBrush(),
                width: 400,
                height: 200,
                enableControls: false,
            });
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("window:fog:close", () => {
            const w = document.body.querySelector(`window-component[window="fog-of-war-settings"]`)
            if (w) w.remove();
        });
        window.addEventListener("window:draw", () => {
            const name = "Draw Settings";
            const window = document.body.querySelector(`window-component[window="draw-settings"]`) || new Window({
                name: name,
                view: new DoodleBrush(),
                width: 400,
                height: 200,
                enableControls: false,
            });
            if (!window.isConnected) {
                document.body.append(window);
            }
        });
        window.addEventListener("window:draw:close", () => {
            const w = document.body.querySelector(`window-component[window="draw-settings"]`)
            if (w) w.remove();
        });
    }
}
const room = new Room();
export default room;
