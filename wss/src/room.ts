import gm from "./game.js";
import type { ExitReason, Socket, Pawn } from "./globals.js";
import { randomUUID } from "crypto";

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
    private cellDistance: number;
    private renderGrid: boolean;
    private fogOfWar: boolean;
    private dmgOverlay: boolean;
    private turnCounter: number;
    private pawns: Pawn[];
    private initiative: Array<Pawn>;
    private activeInitiative: number|null;
    private playerImages: {
        [id:string]: string,
    };
    private fogOfWarShapes: Array<any>;
    private doodleData: any;

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
        this.cellDistance = 5;
        this.renderGrid = false;
        this.fogOfWar = false;
        this.dmgOverlay = false;
        this.turnCounter = 0;
        this.pawns = [];
        this.initiative = [];
        this.activeInitiative = null;
        this.playerImages = {};
        this.fogOfWarShapes = [];

        console.log(`Room ${this.code} created by ${ws.id}`);
        gm.send(ws, "room:create", this.code);
    }

    public syncDoodle(data):void{
        this.doodleData = data;
        this.broadcastPlayers("room:tabletop:doodle", data);
    }

    public syncFog(shape):void{
        if (shape.points.length < 2) {
            console.log("Got malformed fog of war shape:", shape);
            return;
        }
        this.fogOfWarShapes.push(shape);
        this.broadcast("room:tabletop:fog:add", {
            fogOfWarShapes: shape,
            fogOfWar: this.fogOfWar,
        });
    }

    public renamePlayer({ playerId, name }):void{
        if (playerId in this.sockets){
            this.sockets[playerId].name = name;
        }
        this.syncPlayers();
        this.broadcast("room:player:rename", { playerId, name });
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

    public syncInitiative(initiative:Array<Pawn>):void{
        this.initiative = initiative;
        this.broadcast("room:initiative:sync", initiative);
    }

    private decrementConditions() {
        let prev;
        if (this.activeInitiative - 1 < 0) {
            prev = this.initiative[this.initiative.length - 1];
        } else {
            prev = this.initiative[this.activeInitiative - 1];
        }
        const current = this.initiative[this.activeInitiative];

        for (const pawn of this.pawns) {
            let trigger = null;
            if (pawn.uid === current.uid) {
                trigger = "start";
            } else if (pawn.uid === prev.uid){
                trigger = "end";
            }
            if (trigger !== null){
                for (const key in pawn.conditions) {
                    if (pawn.conditions[key].duration >= 0 && pawn.conditions[key].trigger === trigger){
                        pawn.conditions[key].duration--;
                        if (pawn.conditions[key].duration > 0) {
                            this.broadcast("room:tabletop:pawn:status:add", { pawnId: pawn.uid, condition: pawn.conditions[key] });
                        } else if (pawn.conditions[key].duration === 0) {
                            delete pawn.conditions[key];
                            this.broadcast("room:tabletop:pawn:status:remove", { pawnId: pawn.uid, uid: key });
                        }
                    }
                }
            }
        }
    }

    public progressInitiative():void{
        if (this.activeInitiative === null){
            this.activeInitiative = 0;
        } else {
            this.activeInitiative++;
        }
        if (this.activeInitiative >= this.initiative.length) {
            this.activeInitiative = 0;
            this.turnCounter++;
        }
        this.decrementConditions();
        this.broadcast("room:initiative:active", this.activeInitiative);
        this.announceInitiative();
    }

    public setActiveInitiative(index:number):void{
        if (this.activeInitiative === this.initiative.length - 1 && index === 0) {
            this.turnCounter++;
        }
        this.activeInitiative = index;
        this.decrementConditions();
        this.broadcast("room:initiative:active", index);
        this.announceInitiative();
    }

    public clearInitiative():void{
        this.initiative = [];
        this.broadcast("room:initiative:sync", []);
        this.activeInitiative = null;
        this.broadcast("room:initiative:active", null);
        this.broadcast("room:initiative:clear");
        this.turnCounter = 0;
    }

    public async announceInitiative():Promise<void>{
        if (this.initiative.length < 3 || this.activeInitiative === null) return;
        const current = this.initiative[this.activeInitiative];
        let next;
        if (this.activeInitiative + 1 < this.initiative.length) {
            next = this.initiative[this.activeInitiative + 1];
        } else {
            next = this.initiative[0];
        }
        if (!current || !next) return;
        if (current.type === "player" && current.uid in this.sockets){
            gm.send(this.sockets[current.uid], "room:announce:initiative", {
                title: "You're up!",
                message: "It's your turn for combat. Good luck!",
            });
            this.broadcast("room:initiative:turn:start", current.uid);
        } else if (current.type === "monster" && this.gmId in this.sockets) {
            this.broadcast("room:initiative:turn:start", this.gmId);
        } else if (current.type === "npc" && current.ownerId in this.sockets) {
            this.broadcast("room:initiative:turn:start", current.ownerId);
        }
        if (next.type === "player"){
            gm.send(this.sockets[next.uid], "room:announce:initiative", {
                title: "You're on deck.",
                message: "Start planning your turn now. You're next in the initiative order.",
            });
        }
    }

    public setPlayerImage(ws, image:string):void{
        this.playerImages[ws.id] = image;
        this.broadcast("room:tabletop:pawn:image", { pawnId: ws.id, image });
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
        gm.send(this.sockets[this.gmId], "room:tabletop:fog:init");
    }

    public fillFog():void{
        this.fogOfWarShapes = [];
        this.fogOfWar = true;
        this.broadcast("room:tabletop:fog:sync", {
            fogOfWarShapes: this.fogOfWarShapes,
            fogOfWar: this.fogOfWar,
        });
    }

    public clearFog():void{
        this.fogOfWarShapes = [];
        this.fogOfWar = false;
        this.broadcast("room:tabletop:fog:sync", {
            fogOfWarShapes: this.fogOfWarShapes,
            fogOfWar: false,
        });
    }

    public updateMap({ cellSize, renderGrid, cellDistance, prefillFog, dmgOverlay }){
        this.cellSize = cellSize;
        this.renderGrid = renderGrid;
        this.cellDistance = cellDistance;
        this.fogOfWar = prefillFog;
        this.dmgOverlay = dmgOverlay;
        this.broadcast("room:tabletop:map:update", { cellSize, renderGrid, cellDistance, prefillFog, dmgOverlay });
    }

    public clearMap(){
        this.showPawns = false;
        this.pawns = [];
        this.map = "";
        this.fogOfWarShapes = [];
        this.doodleData = "";
        this.initiative = [];
        this.activeInitiative = null;
        this.turnCounter = 0;
        this.broadcast("room:tabletop:clear");
    }

    public spawnNPC({ x, y, name, hp, ac, size, ownerId, image }){
        const id = randomUUID();
        const pawn:Pawn = {
            uid: id,
            x: x,
            y: y,
            hidden: false,
            room: this.code,
            name: name,
            ac: ac,
            hp: hp,
            fullHP: hp,
            size: size,
            image: image || "",
            conditions: {},
            type: "npc",
            ownerId: ownerId,
        };
        this.pawns.push(pawn);
        this.broadcast("room:tabletop:pawn:spawn", pawn);
    }

    public deletePawn(pawnId){
        for (let i = 0; i < this.pawns.length; i++){
            if (this.pawns[i].uid === pawnId){
                this.pawns.splice(i, 1);
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:delete", pawnId);
    }

    public spawnMonster({ monsterId, x, y, name, hp, ac, size, image, hidden }){
        const id = randomUUID();
        const pawn:Pawn = {
            x: x,
            y: y,
            hp: hp,
            ac: ac,
            room: this.code,
            hidden: hidden,
            uid: id,
            name: name,
            fullHP: hp,
            size: size,
            image: image,
            monsterId: monsterId,
            conditions: {},
            type: "monster",
        };
        this.pawns.push(pawn);
        this.broadcast("room:tabletop:pawn:spawn", pawn);
    }

    public setPawnName({ pawnId, name }){
        console.log(`Setting pawn ${pawnId} name to ${name}`);
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                pawn.name = name;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:rename", { pawnId, name });
    }

    public setPawnHealth({ pawnId, hp }){
        console.log(`Setting pawn ${pawnId} to ${hp} HP`);
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                pawn.hp = hp;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:health", { pawnId, hp });
    }
    
    public setPawnAC({ pawnId, ac }){
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                pawn.ac = ac;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:ac", { pawnId, ac });
    }

    public setPawnVisibility({ pawnId, hidden }){
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                pawn.hidden = hidden;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:visibility", { pawnId, hidden });
    }

    public setPawnCondition({ pawnId, uid, name, color, duration, trigger }){
        const condition = {
            uid: uid,
            name: name,
            color: color,
            duration: duration,
            trigger: trigger,
        };
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                pawn.conditions[uid] = condition;
                break;
            }
        }
        for (const pawn of this.initiative) {
            if (pawn.uid === pawnId){
                pawn.conditions[uid] = condition;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:status:add", { pawnId: pawnId, condition: condition });
    }

    public removePawnCondition({ pawnId, uid }){
        for (const pawn of this.pawns){
            if (pawn.uid === pawnId){
                delete pawn.conditions[uid];
                break;
            }
        }
        for (const pawn of this.initiative){
            if (pawn.uid === pawnId){
                delete pawn.conditions[uid];
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:status:remove", { pawnId, uid });
    }

    public movePawn({ uid, x, y }){
        for (const pawn of this.pawns){
            if (pawn.uid === uid){
                console.log(`Moving pawn ${uid} to ${x}, ${y}`);
                pawn.x = x;
                pawn.y = y;
                break;
            }
        }
        this.broadcast("room:tabletop:pawn:move", { uid, x, y });
    }

    public syncPawns(ws:Socket){
        for (const pawn of this.pawns){
            gm.send(ws, "room:tabletop:pawn:spawn", pawn);
        }
    }

    public spawnPlayers(){
        this.showPawns = true;
        for (const id in this.sockets){
            if (id !== this.gmId){
                const pawn:Pawn = {
                    x: 0,
                    y: 0,
                    room: this.code,
                    hidden: false,
                    uid: id,
                    name: this.sockets[id].name,
                    image: this.playerImages?.[id] || "",
                    conditions: {},
                    type: "player",
                    hp: this.sockets[id].hp,
                    fullHP: this.sockets[id].maxHP,
                    ac: this.sockets[id].ac,
                };
                this.pawns.push(pawn);
                this.broadcast("room:tabletop:pawn:spawn", pawn);
            }
        }
    }

    public broadcast(type: string, data = null):void{
        for (const socket in this.sockets){
            gm.send(this.sockets[socket], type, data);
        }
    }

    public broadcastPlayers(type: string, data = null):void{
        for (const socket in this.sockets){
            if (socket !== this.gmId){
                gm.send(this.sockets[socket], type, data);
            }
        }
    }

    public resetSocket(ws:Socket){
        ws.room = this.code;
        this.sockets[ws.id] = ws;
        ws.name = this.deadSockets[ws.id].name;
        delete this.deadSockets[ws.id];
        console.log(`Socket ${ws.id} reconnected to ${this.code}`);
        gm.send(ws, "room:joined", this.gmId === ws.id ? this.code : null);
        this.broadcast("room:announce:reconnect", `${ws.name} has returned.`);
        this.syncPlayers();
        this.syncMap(ws);
        this.syncPawns(ws);
        gm.send(ws, "room:initiative:sync", this.initiative);
        gm.send(ws, "room:initiative:active", this.activeInitiative);
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
        this.broadcast("room:announce:join", `${ws.name} joined the room.`);
        gm.send(ws, "room:joined", this.gmId === ws.id ? this.code : null);
        if (this.showPawns){
            const pawn:Pawn = {
                x: 0,
                y: 0,
                room: this.code,
                hidden: false,
                uid: ws.id,
                name: ws.name,
                image: this.playerImages?.[ws.id] || "",
                conditions: {},
                type: "player",
                hp: ws.hp,
                fullHP: ws.maxHP,
                ac: ws.ac,
            };
            this.pawns.push(pawn);
            this.broadcast("room:tabletop:pawn:spawn", pawn);
        }
        ws.room = this.code;
        this.sockets[ws.id] = ws;
        console.log(`Socket ${ws.id} joined room ${this.code}`);
        this.syncPlayers();
        this.syncMap(ws);
        this.syncPawns(ws);
    }

    private syncMap(ws:Socket){
        if (this.map !== ""){
            gm.send(ws, "room:tabletop:load", this.map);
            const cellSize = this.cellSize;
            const renderGrid = this.renderGrid;
            const cellDistance = this.cellDistance;
            const fogOfWar = this.fogOfWar;
            const dmgOverlay = this.dmgOverlay;
            gm.send(ws, "room:tabletop:map:update", { cellSize, renderGrid, cellDistance, fogOfWar, dmgOverlay });
            gm.send(ws, "room:tabletop:fog:sync", {
                fogOfWarShapes: this.fogOfWarShapes,
                fogOfWar: this.fogOfWar,
            });
            gm.send(ws, "room:tabletop:doodle", this.doodleData);
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
                ac: this.sockets[id].ac,
                hp: this.sockets[id].hp,
                maxHP: this.sockets[id].maxHP,
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
