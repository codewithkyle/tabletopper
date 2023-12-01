import * as uws from "../uws/uws.js";
import dotenv from "dotenv";
import gm from "./game.js";
import { TextDecoder } from "util";

const decoder = new TextDecoder("utf-8");
dotenv.config();
const port = process.env.PORT;
let app;
if (process.env.ENV === "dev"){
    // @ts-ignore
    app = uws.App();
}
else {
    // @ts-ignore
    app = uws.SSLApp({
        key_file_name: process.env.KEY,
        cert_file_name: process.env.CERT,
    });
}

app.ws("/*", {
    // @ts-ignore
    compression: uws.SHARED_COMPRESSOR,
    maxPayloadLength: 150 * 1024 * 1024,
    maxBackpressure: 150 * 1024 * 1024,
    idleTimeout: 32,
    open: (ws) => {
        gm.connect(ws);
    },
    message: (ws, message, isBinary) => {
        try{
            gm.message(ws, JSON.parse(decoder.decode(message)));
        } catch (e){
            // Log error
        }
    },
    close: (ws) => {
        gm.disconnect(ws);
    },
});

app.listen("127.0.0.1", port, {}, (token) => {});
