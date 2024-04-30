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
    ownerId?: string,
}

export interface Condition {
    uid: string,
    name: string,
    color: "blue" | "green" | "orange" | "pink" | "purple" | "red" | "white" | "yellow",
    duration: number,
    trigger: "start" | "end",
}
