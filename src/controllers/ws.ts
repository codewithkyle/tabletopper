import { publish } from "@codewithkyle/pubsub";
import notifications from "~brixi/controllers/notifications";

let socket;
let connected = false;
let wasReconnection = false;

async function connect() {
    if (connected){
        return;
    }
    // @ts-expect-error
    const { SOCKET_URL } = await import("/config.js");
    try{
        socket = new WebSocket(SOCKET_URL);
    } catch (e) {
        console.error(e);
    }
    socket.addEventListener("message", (event) => {
        try {
            const data = JSON.parse(event.data);
            publish("socket", {
                type: "message",
                data: data,
            });
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
export { connected, disconnect, connect };