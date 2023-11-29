export type Socket = {
    id: string,
    room: string,
    send: Function,
    name: string,
};

export type ExitReason = "UNKNOWN" | "KICKED" | "DC" | "QUIT";

export type Initiative = {
    uid: string,
    name: string,
    type: "player"|"monster"|"npc",
    index: number,
}

export type Pawn = {
    uid: string,
    x: number,
    y: number,
    token?: string|null,
    name: string,
    room: string,
    hp?: number,
    ac?: number,
    hidden: boolean,
    image: string,
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
    type: "player"|"monster"|"npc",
    monsterId?: string,
}
