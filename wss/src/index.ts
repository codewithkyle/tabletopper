import * as uws from "../uws/uws.js";
import gm from "./game.js";
import { TextDecoder } from "util";

const decoder = new TextDecoder("utf-8");
let app = uws.App();

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

app.listen("127.0.0.1", 8080, {}, (token) => {});
