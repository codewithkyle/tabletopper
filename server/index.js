const uws = require("./uws/uws");
require("dotenv").config({ path: "./.env" });

const port = process.env.PORT;
const app = uws.App();

app.ws("/*", {
    open: (ws) => {
        console.log("Socket connected");
    },
    message: (ws, message, isBinary) => {
        /* Here we echo the message back, using compression if available */
        let ok = ws.send(message, isBinary, true);
    },
    drain: (ws) => {
        console.log('WebSocket backpressure: ' + ws.getBufferedAmount());
    },
    close: (ws) => {
        console.log("Socket closed");
    },
});

app.listen(port, (token) => {
    if (token){
        console.log(`Listening on port: ${port}`);
    }
    else {
        console.log(`Failed to start on port: ${port}`);
    }
});