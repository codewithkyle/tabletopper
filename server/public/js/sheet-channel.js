export function sheetChannel() {
    if (typeof BroadcastChannel !== "function") {
        return null;
    }
    return new BroadcastChannel("tabletopper:sheet");
}
