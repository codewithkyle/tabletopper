import { subscribe } from "@codewithkyle/pubsub";
import { connect, send } from "~controllers/ws";

class Room {
    private uid: string;
    private room: string;
    private character: string;
    private isGM: boolean;

    constructor() {
        this.uid = "";
        this.room = "";
        this.character = "";
        this.isGM = false;
        subscribe("socket", this.inbox.bind(this));
        this.init();
    }

    private async inbox({ type, data }){
        switch (type){
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
                    console.log(room);
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
                break;
            case "room:joined":
                if (data){
                    fetch(`/session/gm/${data}`, {
                        method: "POST",
                    });
                } else {
                    fetch(`/session/gm`, {
                        method: "DELETE",
                    });
                }
                break;
            default:
                break;
        }
    }

    async init(){
        connect();
    }
}
const room = new Room();
export default room;
