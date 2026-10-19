import { ROOM_CHARACTER } from "./events.js";
import { sheetChannel } from "./sheet-channel.js";

const channel = sheetChannel();
channel?.addEventListener("message", (e) => {
    const id = e.data?.id;
    if (typeof id === "string") {
        window.dispatchEvent(new CustomEvent(ROOM_CHARACTER, { detail: { id } }));
    }
});
