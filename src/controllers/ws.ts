import { toast } from "@codewithkyle/notifyjs";

let socket;
let connected = false;

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
        // TODO: sync state
    });
}
reconnect();

function disconnect() {
    if (connected) {
        toast({
            title: "Connection Lost",
            message:
                "You've lost your connection with the server. Any changes you make will be applied when you've reconnected.",
            classes: ["-yellow"],
            closeable: true,
            duration: Infinity,
        });
        connected = false;
    }
    setTimeout(() => {
        reconnect();
    }, 5000);
}
export { connected, disconnect };