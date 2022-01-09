const uws = require("./uws/uws.js");
const path = require("path");
require("dotenv").config({ path: path.join(__dirname, ".env") });

const port = process.env.PORT;
let app;
if (process.env.ENV === "dev"){
    app = uw.App();
}
else {
    app = uws.SSLApp({
        key_file_name: process.env.KEY,
        cert_file_name: process.env.CERT,
    });
}

app.ws("/*", {
    compression: uws.SHARED_COMPRESSOR,
    maxPayloadLength: 16 * 1024 * 1024,
    idleTimeout: 32,
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

app.listen("0.0.0.0", port, {}, (token) => {});