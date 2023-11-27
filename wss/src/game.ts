import type { Socket } from "./globals.js";
import Room from "./room.js";
import { GenerateCode } from "./utils.js";

class GameManager {
    private sockets: {
        [id:string]: Socket,
    };
    private rooms: {
        [code:string]: Room,
    }

    constructor(){
        this.sockets = {};
        this.rooms = {};
    }

    public connect(ws):void{
        //this.sockets.push(ws);
    }

    public disconnect(ws:Socket):void{
        if (ws.id in this.sockets){
            console.log(`Socket ${ws.id} disconnected`);
            if (ws.room && ws.room in this.rooms){
                this.rooms[ws.room].removeSocket(ws, "DC");
            }
            delete this.sockets[ws.id];
        }
    }

    public message(ws:Socket, message):void{
        const { type, data } = message;
        const room = this.rooms?.[ws?.room] || this.rooms?.[data?.room] || null;
        switch (type){
            case "room:player:rename":
                if (room){
                    room.renamePlayer(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:player:mute":
                if (room){
                    room.mutePlayer(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:ping":
                if (room){
                    room.ping(data, ws);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:announce:initiative":
                if (room){
                    room.announceInitiative(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:spawn:npc":
                if (room){
                    room.spawnNPC(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:spawn:monster":
                if (room){
                    room.spawnMonster(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:player:ban":
                if (room){
                    room.kickPlayer(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:map:clear":
                if (room){
                    room.clearMap();
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:map:load":
                if (room){
                    room.loadMap(data);
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:tabletop:spawn:players":
                if (room){
                    room.spawnPlayers();
                } else {
                    this.error(ws, "Action Failed", `Room ${ws.room} is no longer available.`);
                }
                break;
            case "room:quit":
                if (room){
                    room.removeSocket(ws, "QUIT");
                }
                break;
            case "room:join":
                if (room){
                    ws.id = data.id;
                    ws.name = data.name;
                    this.sockets[ws.id] = ws;
                    room.addSocket(ws);
                }
                else {
                    this.error(ws, "Failed to Join", `Room ${data.room} is no longer available.`);
                    this.send(ws, "room:exit");
                }
                break;
            case "room:unlock":
                if (room){
                    room.unlock(ws);
                } else {
                    this.error(ws, "Action Failed", "You cannot unlock a room that doesn't exist.");
                }
                break;
            case "room:lock":
                if (room){
                    room.lock(ws);
                } else {
                    this.error(ws, "Action Failed", "You cannot lock a room that doesn't exist.");
                }
                break;
            case "room:create":
                ws.id = data;
                ws.name = "Game Master";
                this.sockets[ws.id] = ws;
                this.createRoom(ws); 
                break;
            default:
                // TODO: log unexpected message types
                break;
        }
    }

    public send(ws:Socket, type:string, data:any = null):void{
        ws.send(JSON.stringify({
            type: type,
            data: data,
        }));
    }

    public error(ws:Socket, title:string, message:string):void{
        this.send(ws, "core:error", {
            title: title,
            message: message,
        });
    }

    public async createRoom(ws:Socket):Promise<void>{
        let code:string;
        do {
            code = GenerateCode();
        } while (code in this.rooms)
        ws.room = code;
        const room = new Room(code, ws);
        this.rooms[code] = room;
        console.log(`Socket ${ws.id} created room ${code}`);
    }

    public removeRoom(code:string):void{
        delete this.rooms[code];
        console.log(`Room ${code} was removed`);
    }
}
const gm = new GameManager();
export default gm;
