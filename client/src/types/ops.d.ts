export type OP = "INSERT" | "DELETE" | "SET" | "UNSET" | "BATCH";

export interface OPCode{
    uid: string,
    table: string,
    op: OP,
    timestamp: number,
    key: string,
}

export interface Insert extends OPCode{
    value: any,
}

export interface Delete extends OPCode{}

export interface Set extends OPCode{
    keypath: string,
    value: any,
}

export interface Unset extends OPCode{
    keypath: string,
}

export interface Batch extends OPCode{
    ops: Array<OPCode>,
}
