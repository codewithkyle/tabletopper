import { publish } from "@codewithkyle/pubsub";
import { navigateTo } from "@codewithkyle/router";
import cc from "./control-center";
import db from "@codewithkyle/jsql";
import alerts from "~brixi/controllers/alerts";

let socket:WebSocket;
let connected = false;
let wasReconnection = false;

async function connect() {
    if (connected){
        return;
    }
    // @ts-expect-error
    const { SOCKET_URL, ENV } = await import("/config.js");
    try{
        socket = new WebSocket(SOCKET_URL);
    } catch (e) {
        console.error(e);
    }
    socket.addEventListener("message", async (event) => {
        try {
            const { type, data } = JSON.parse(event.data);
            if (ENV === "dev"){
                console.log(type, data);
            }
            switch(type){
                case "room:tabletop:ping":
                    publish("tabletop", {
                        type: "ping",
                        data: data,
                    });
                    break;
                case "room:tabletop:clear":
                    //await db.query("RESET ledger");
                    await db.query("RESET pawns");
                    publish("tabletop", {
                        type: "clear",
                    });
                    break;
                case "room:ban":
                    sessionStorage.removeItem("room");
                    sessionStorage.removeItem("lastSocketId");
                    sessionStorage.setItem("kicked", "true");
                    location.href = location.origin;
                    break;
                case "room:join":
                    sessionStorage.setItem("room", data.uid);
                    sessionStorage.setItem("lastSocketId", sessionStorage.getItem("socketId"));
                    publish("socket", {
                        type: type,
                        data: data,
                    });
                    break;
                case "room:announce:snackbar":
                    alerts.snackbar(data);
                    break;
                case "room:announce:unlock":
                    alerts.toast("The room has been unlocked.");
                    break;
                case "room:announce:lock":
                    alerts.toast("The room has been locked.");
                    break;
                case "room:announce:reconnect":
                    alerts.success("Player Reconnected", data);
                    break;
                case "room:announce:initiative":
                    alerts.alert(data.title, data.message, [], 30);
                    break;
                case "room:announce:kick":
                    alerts.alert("Player Kicked", data);
                    break;
                case "room:announce:quit":
                    alerts.alert("Player Left", data);
                    break;
                case "room:announce:dc":
                    alerts.warn("Player Disconnected", data);
                    break;
                case "room:announce:join":
                    alerts.success("Player Joined", data);
                    break;
                case "room:op":
                    await cc.perform(data);
                    publish("sync", data);
                    break;
                case "core:error":
                    alerts.error(data.title, data.message);
                    break;
                case "core:init":
                    sessionStorage.setItem("socketId", data.id);
                    break;
                case "core:sync:fail":
                    sessionStorage.removeItem("room");
                    sessionStorage.removeItem("lastSocketId");
                    navigateTo("/");
                    break;
                default:
                    publish("socket", {
                        type: type,
                        data: data,
                    });
                    break;
            }
        } catch (e) {
            console.error(e, event);
        }
    });
    socket.addEventListener("close", () => {
        disconnect();
    });
    socket.addEventListener("open", () => {
        connected = true;
        if (wasReconnection){
            alerts.success("Reconnected", "We've reconnected with the server.");
        }
        wasReconnection = false;
        publish("socket", {
            type: "connected",
        });
    });
}

function disconnect() {
    publish("socket", {
        type: "disconnected",
    });
    if (connected) {
        alerts.warn("Connection Lost", "Hang tight we've lost the server connection. Any changes you make will be synced when you've reconnected.");
        connected = false;
        wasReconnection = true;
    }
    setTimeout(() => {
        connect();
    }, 5000);
}

function send(type:string, data:any = null):void{
    const message = JSON.stringify({
        type: type,
        data: data,
    });
    if (connected){
        socket.send(message);
    }
    else if (!connected && wasReconnection){
        // We lost connection, check if we were in a game
    }
    else {
        // Do... something... maybe?
    }
}

function close():void{
    if (connected){
        socket.close();
    }
}

export { connected, disconnect, connect, send, close };