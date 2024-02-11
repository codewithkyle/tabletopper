import { publish } from "@codewithkyle/pubsub";
import alerts from "~brixi/controllers/alerts";

let socket:WebSocket;
let connected = false;
let wasReconnection = false;

let offlineQueue = [];
function flushOfflineQueue(){
    while (offlineQueue.length > 0){
        const {type, data} = offlineQueue.shift();
        send(type, data);
    }
}

async function connect() {
    if (connected){
        return;
    }
    // @ts-expect-error
    const { SOCKET_URL } = await import("/static/config.js");
    socket = new WebSocket(SOCKET_URL);
    socket.addEventListener("message", async (event) => {
        try {
            const { type, data } = JSON.parse(event.data);
            switch(type){
                case "room:exit":
                    setTimeout(() => {
                        location.href = location.origin;
                    }, 3000);
                    break;
                case "room:ban":
                    location.href = location.origin;
                    break;
                case "room:announce:toast":
                    alerts.toast(data);
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
                    alerts.success("Player Reconnected", data, [], 10);
                    break;
                case "room:announce:initiative":
                    alerts.alert(data.title, data.message, [], 10);
                    break;
                case "room:announce:kick":
                    alerts.alert("Player Kicked", data, [], 10);
                    break;
                case "room:announce:quit":
                    alerts.alert("Player Left", data, [], 10);
                    break;
                case "room:announce:dc":
                    alerts.warn("Player Disconnected", data, [], 10);
                    break;
                case "room:announce:join":
                    alerts.success("Player Joined", data, [], 10);
                    break;
                case "core:error":
                    alerts.error(data.title, data.message, [], 10);
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
            alerts.success("Reconnected", "We've reconnected with the server.", [], 10);
            flushOfflineQueue();
        }
        wasReconnection = false;
        publish("socket", {
            type: "core:init",
            data: null,
        });
    });
}

function disconnect() {
    if (connected) {
        alerts.warn("Connection Lost", "Hang tight we've lost the server connection. Any changes you make will be synced when you've reconnected.", [], 10);
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
    else if (wasReconnection){
        console.log("Queued message", type, data);
        offlineQueue.push({
            type: type,
            data: data,
        });
    } else {
        console.error("Failed to send message", type, data, connected, wasReconnection, socket);
    }
}

function close():void{
    if (connected){
        socket.close();
    }
}

export { connected, disconnect, connect, send, close };
