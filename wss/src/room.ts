import gm from "./game.js";
import type { ExitReason, Socket, Pawn } from "./globals.js";
import { randomUUID } from "crypto";

const COLORS = ["grey", "red", "orange", "amber", "yellow", "lime", "green", "emerald", "teal", "cyan", "light-blue", "blue", "indigo", "violet", "purple", "fuchsia", "pink", "rose"];

class Room {
    public code: string;
    public gmId: string;
    private sockets: {
        [id:string]: Socket,
    };
    public locked:boolean;
    private deadSockets: {
        [id:string]: {
            name: string,
        }
    };
    public showPawns: boolean;
    private mutedPlayers: {
        [id:string]: null,
    };
    private map: string;
    private cellSize: number;
    private renderGrid: boolean;

    constructor(code:string, ws:Socket){
        this.code = code;
        this.gmId = ws.id;
        this.sockets = {};
        this.sockets[ws.id] = ws;
        this.locked = false;
        this.deadSockets = {};
        this.showPawns = false;
        this.mutedPlayers = {};
        this.map = "";
        this.cellSize = 32;
        this.renderGrid = false;

        console.log(`Room ${this.code} created by ${ws.id}`);
        gm.send(ws, "room:create", this.code);
    }

    public renamePlayer({ playerId, name }):void{
        if (playerId in this.sockets){
            this.sockets[playerId].name = name;
        }
        this.syncPlayers();
    }

    public mutePlayer({ playerId }): void{
        let muted = false;
        if (playerId in this.mutePlayer){
            delete this.mutePlayer[playerId];
        } else {
            muted = true;
            this.mutePlayer[playerId] = null;
        }
        gm.send(this.sockets[playerId], "room:announce:snackbar", `The Game Master has ${muted ? "muted" : "unmuted"} your pings.`);
        gm.send(this.sockets[this.gmId], "room:announce:toast", `${this.sockets[playerId].name} is now ${muted ? "muted" : "unmuted"}.`);
    }

    public ping(data, ws):void{
        if (ws.id in this.mutePlayer) return;
        this.broadcast("room:tabletop:ping", data);
    }

    public async announceInitiative({ current, next }):Promise<void>{
        //const op = set("games", this.code, "active_initiative", current.uid);
        //await this.dispatch(op);
        if (current?.playerId != null){
            gm.send(this.sockets[current.playerId], "room:announce:initiative", {
                title: "You're up!",
                message: "It's your turn for combat. Good luck!",
            });
        }
        if (next.playerId != null){
            gm.send(this.sockets[next.playerId], "room:announce:initiative", {
                title: "You're on deck.",
                message: "Start planning your turn now. You're next in the initiative order.",
            });
        } else {
            this.broadcast("room:announce:initiative", {
                title: `${next.name} is on deck.`,
                message: `${next.name} is next in the initiative order.`,
            });
        }
    }
    
    public kickPlayer(id:string):void{
        if(id in this.sockets){
            gm.send(this.sockets[id], "room:ban");
            this.removeSocket(this.sockets[id], "KICKED");
        }
    }

    public loadMap(map:string):void{
        this.map = map;
        this.broadcast("room:tabletop:load", map);
    }

    public updateMap({ cellSize, renderGrid }){
        this.cellSize = cellSize;
        this.renderGrid = renderGrid;
        this.broadcast("room:tabletop:map:update", { cellSize, renderGrid });
    }

    public clearMap(){
        this.broadcast("room:tabletop:clear");
        this.showPawns = false;
        this.map = "";
    }

    public spawnNPC({ name, ac, hp, x, y, size }){
        const id = randomUUID();
        const pawn:Pawn = {
            uid: id,
            x: x,
            y: y,
            room: this.code,
            name: name,
            ac: ac,
            hp: hp,
            fullHP: hp,
            size: size,
            rings: {
                red: false,
                orange: false,
                blue: false,
                white: false,
                purple: false,
                yellow: false,
                pink: false,
                green: false,
            },
        };
        //const op = insert("pawns", id, pawn);
        //console.log(`Room ${this.code} is spawning an NPC named ${name}`);
        //await this.dispatch(op);
    }

    public spawnMonster({ index, x, y, name, hp, ac, size }){
        const id = randomUUID();
        const pawn:Pawn = {
            uid: id,
            x: x,
            y: y,
            hp: hp,
            ac: ac,
            room: this.code,
            monsterId: index,
            name: name,
            fullHP: hp,
            size: size,
            rings: {
                red: false,
                orange: false,
                blue: false,
                white: false,
                purple: false,
                yellow: false,
                pink: false,
                green: false,
            },
        };
        //const op = insert("pawns", id, pawn);
        //console.log(`Room ${this.code} is spawning a ${index}`);
        //await this.dispatch(op);
    }

    public spawnPlayers(){
        this.showPawns = true;
        const ids = [];
        for (const id in this.sockets){
            if (id !== this.gmId){
                ids.push(id);
                const pawn:Pawn = {
                    uid: randomUUID(),
                    x: 0,
                    y: 0,
                    room: this.code,
                    playerId: id,
                    name: this.sockets[id].name,
                    rings: {
                        red: false,
                        orange: false,
                        blue: false,
                        white: false,
                        purple: false,
                        yellow: false,
                        pink: false,
                        green: false,
                    },
                };
                //await this.dispatch(insert("pawns", id, pawn));
            }
        }
        //console.log(`Room ${this.code} is spawning players`);
        //const op = set("games", this.code, "players", ids);
        //await this.dispatch(op);
    }

    public broadcast(type: string, data = null):void{
        for (const socket in this.sockets){
            gm.send(this.sockets[socket], type, data);
        }
    }

    public async resetSocket(ws:Socket):Promise<void>{
        ws.room = this.code;
        this.sockets[ws.id] = ws;
        ws.name = this.deadSockets[ws.id].name;
        delete this.deadSockets[ws.id];
        console.log(`Socket ${ws.id} reconnected to ${this.code}`);
        gm.send(ws, "room:joined", this.gmId === ws.id ? this.code : null);
        this.broadcast("room:announce:reconnect", `${ws.name} has returned.`);
        this.syncPlayers();
        this.syncMap(ws);
    }

    public addSocket(ws:Socket){
        if (ws.id in this.deadSockets){
            return this.resetSocket(ws);
        }
        if (this.locked){
            gm.error(ws, "Room Locked", "This room is locked. You cannot join.");
            gm.send(ws, "room:exit");
            return;
        }
        ws.room = this.code;
        this.sockets[ws.id] = ws;
        console.log(`Socket ${ws.id} joined room ${this.code}`);
        this.broadcast("room:announce:join", `${ws.name} joined the room.`);
        gm.send(ws, "room:joined");
        this.syncPlayers();
        this.syncMap(ws);
    }

    private syncMap(ws:Socket){
        if (this.map !== ""){
            gm.send(ws, "room:tabletop:load", this.map);
            const cellSize = this.cellSize;
            const renderGrid = this.renderGrid;
            gm.send(ws, "room:tabletop:map:update", { cellSize, renderGrid });
        }
    }

    private syncPlayers(){
        const players = [];
        for (const id in this.sockets){
            players.push({
                uid: id,
                name: this.sockets[id].name,
                gm: id === this.gmId,
                muted: id in this.mutedPlayers,
            });
        }
        this.broadcast("room:sync:players", players);
    }

    public async removeSocket(ws:Socket, reason: ExitReason = "UNKNOWN"):Promise<void>{
        if (ws.id in this.sockets){
            this.deadSockets[ws.id] = {
                name: ws.name,
            };
            delete this.sockets[ws.id];
            console.log(`Socket ${ws.id} left room ${this.code}`);
        }
        switch (reason){
            case "QUIT":
                this.broadcast("room:announce:quit", `${ws.name} has left the room.`);
                break;
            case "KICKED":
                this.broadcast("room:announce:kick", `${ws.name} has been kicked from the room.`);
                break;
            case "DC":
                this.broadcast("room:announce:dc", `${ws.name} was disconnected.`);
                break;
            default:
                break;
        }
        this.syncPlayers();
        if (Object.keys(this.sockets).length === 0){
            gm.removeRoom(this.code);
        }
    }

    public lock(ws:Socket):void{
        if (ws.id === this.gmId){
            this.locked = true;
            gm.send(this.sockets[this.gmId], "room:announce:lock");
        }
        else {
            gm.error(ws, "Action Failed", "Only the Game Master can lock the room.");
        }
    }

    public unlock(ws:Socket):void{
        if (ws.id === this.gmId){
            this.locked = false;
            gm.send(this.sockets[this.gmId], "room:announce:unlock");
        }
        else {
            gm.error(ws, "Action Failed", "Only the Game Master can unlock the room.");
        }
    }
}
export default Room;
