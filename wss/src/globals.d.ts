export type Socket = {
    id: string,
    room: string,
    send: Function,
    name: string,
    image: string,
    maxHP: number,
    hp: number,
    ac: number,
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
    hp: number,
    ac: number,
    hidden: boolean,
    image: string,
    conditions: {
        [uid: string]: Condition,
    },
    fullHP: number,
    size?: string|null,
    type: "player"|"monster"|"npc",
    monsterId?: string,
}

export interface Condition {
    uid: string,
    name: string,
    color: "blue" | "green" | "orange" | "pink" | "purple" | "red" | "white" | "yellow",
    duration: number,
}
