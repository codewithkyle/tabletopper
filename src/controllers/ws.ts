import notifications from "~brixi/controllers/notifications";

let socket;
let connected = false;
let wasReconnection = false;

async function reconnect() {
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
            console.log(data);
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
        // TODO: sync state
    });
}
reconnect();

function disconnect() {
    if (connected) {
        notifications.warn("Connection Lost", "Hang tight we've lost the server connection. Any changes you make will be synced when you've reconnected.");
        connected = false;
        wasReconnection = true;
    }
    setTimeout(() => {
        reconnect();
    }, 5000);
}
export { connected, disconnect };