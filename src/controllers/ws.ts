import { publish } from "@codewithkyle/pubsub";
import notifications from "~brixi/controllers/notifications";

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
    socket.addEventListener("message", (event) => {
        try {
            const { type, data } = JSON.parse(event.data);
            if (ENV === "dev"){
                console.log(type, data);
            }
            switch(type){
                case "core:init":
                    const prevId = sessionStorage.getItem("socketId");
                    if (prevId != null){
                        sessionStorage.setItem("lastSocketId", prevId);
                    }
                    sessionStorage.setItem("socketId", data.id);
                    send("core:sync", {
                        prevId: prevId,
                        room: sessionStorage.getItem("room") || null,
                    });
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
            notifications.success("Reconnected", "We've reconnected with the server.");
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
        notifications.warn("Connection Lost", "Hang tight we've lost the server connection. Any changes you make will be synced when you've reconnected.");
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
export { connected, disconnect, connect, send };