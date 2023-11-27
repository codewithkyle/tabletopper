export type Socket = {
    id: string,
    room: string,
    send: Function,
    name: string,
};

export type ExitReason = "UNKNOWN" | "KICKED" | "DC" | "QUIT";

export type Pawn = {
    x: number,
    y: number,
    uid: string,
    playerId?: string|null,
    monsterId?: string|null,
    token?: string|null,
    name: string,
    room: string,
    hp?: number,
    ac?: number,
    rings: {
        red: boolean,
        orange: boolean,
        blue: boolean,
        white: boolean,
        purple: boolean,
        yellow: boolean,
        pink: boolean,
        green: boolean,
    },
    fullHP?: number,
    size?: string|null,
}
