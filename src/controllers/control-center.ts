import type { Batch, Delete, Insert, OPCode, Set, Unset } from "~types/ops";
import { createSubscription, publish } from "@codewithkyle/pubsub";
import { setValueFromKeypath, unsetValueFromKeypath } from "~utils/object";
import { connected, send } from "~controllers/ws";
import db from "@codewithkyle/jsql";
import { orderedUUID as uuid } from "@codewithkyle/uuid";

class ControlCenter {
    private syncing: boolean;

    constructor(){
        this.syncing = false;
        createSubscription("sync");
        this.flushOutbox();
    }

    private async flushOutbox(){
        // TODO: get pendings messages from outbox table
        // const messages:Array<any> = [];
        // const successes = [];
        // if (messages.length){
        //     for (const message of messages){
        //         const success = await this.disbatch(message.opcode, true);
        //         if (success){
        //             successes.push(message.uid);
        //         }
        //     }
        // }
        // for (const uid of successes){
        //     // TODO: delete successful messages from outbox table
        // }
        // setTimeout(this.flushOutbox.bind(this), 30000);
    }

    public async sync(){
        if (this.syncing){
            return;
        }
        this.syncing = true;
        try {
            // TODO: figure out if we need to sync with a cloud diagram
        } catch (e) {
            console.error(e);
        }
        this.syncing = false;
    }

    public async runHistory(){
        // Get all past operations
        const ops = await db.query("SELECT * FROM ledger ORDER BY timestamp");
        // Perform ops
        for (const op of ops){
            await this.op(op);
            publish("sync", op);
        }
    }

    public insert(table:string, key:string, value:any):Insert{
        return {
            uid: uuid(),
            op: "INSERT",
            table: table,
            key: key,
            value: value,
            timestamp: new Date().getTime(),
        };
    }

    public delete(table:string, key:string):Delete{
        return {
            uid: uuid(),
            op: "DELETE",
            table: table,
            key: key,
            timestamp: new Date().getTime(),
        };
    }

    public set(table:string, key:string, keypath:string, value:any):Set{
        return {
            uid: uuid(),
            op: "SET",
            table: table,
            key: key,
            keypath: keypath,
            value: value,
            timestamp: new Date().getTime(),
        };
    }

    public unset(table:string, key:string, keypath:string):Unset{
        return {
            uid: uuid(),
            op: "UNSET",
            table: table,
            key: key,
            keypath: keypath,
            timestamp: new Date().getTime(),
        };
    }

    public batch(table:string, key:string, ops:Array<OPCode>):Batch{
        return {
            table: table,
            uid: uuid(),
            op: "BATCH",
            ops: ops,
            timestamp: new Date().getTime(),
            key: key,
        };
    }

    public async dispatch(op:OPCode, bypassOutbox = false){
        if (connected){
            try{
                send("room:op", op);
            } catch (e) {
                console.error(e);
                if (!bypassOutbox){
                    await db.query("INSERT INTO outbox VALUES ($op)", {
                        uid: uuid(),
                        opcode: op,
                    });
                }
            }
        }
    }

    public async perform(operation:OPCode, disbatchToUI = false){
        // @ts-ignore
        const { op, uid, table, key, value, keypath, timestamp } = operation;

        // Ignore web socket OPs if they originated from this client
        const results = await db.query("SELECT * FROM ledger WHERE uid = $uid", {
            uid: uid,
        });
        const alreadyInLedger = results.length > 0;
        if (alreadyInLedger){
            return;
        }

        // Insert OP into the ledger
        await db.query("INSERT INTO ledger VALUES ($op)", {
            op: operation,
        });

        // Get all past operations & select ops to be performed based on new op timestamp
        const ops = await db.query("SELECT * FROM ledger WHERE table = $table AND key = $key AND timestamp >= $timestamp ORDER BY timestamp", {
            table: table,
            key: key,
            timestamp: timestamp,
        });

        // Perform ops
        for (const op of ops){
            await this.op(op);
            if (disbatchToUI){
                publish("sync", op);
            }
        }
    }

    public async op(operation):Promise<any>{
        // @ts-ignore
        const { op, uid, table, key, value, keypath, timestamp, ops } = operation;

        const results = await db.query("SELECT * FROM $table WHERE uid = $uid", {
            uid: key,
            table: table,
        });
        const existingModel = results?.[0] ?? null;
        const alreadyExists = results.length > 0;

        // Skip inserts when we already have the data && skip non-insert ops when we don't have the data
        if (alreadyExists && op === "INSERT" || existingModel === null && op !== "INSERT"){
            return;
        }

        if (op === "BATCH"){
            let doUpdate = false;
            for (const batchOP of operation.ops){
                const type = batchOP.op;
                switch (type){
                    case "INSERT":
                        await db.query("INSERT INTO $table VALUES ($value)", {
                            value: value,
                            table: table,
                        });
                        break;
                    case "DELETE":
                        await db.query("DELETE FROM $table WHERE uid = $uid", {
                            uid: key,
                            table: table,
                        });
                        break;
                    case "SET":
                        setValueFromKeypath(existingModel, batchOP.keypath, batchOP.value);
                        doUpdate = true;
                        break;
                    case "UNSET":
                        unsetValueFromKeypath(existingModel, batchOP.keypath);
                        doUpdate = true;
                        break;
                    default:
                        console.error(`Unknown OP: ${op}`);
                        break;
                }
            }
            if (doUpdate){
                await db.query("UPDATE $table SET $value WHERE uid = $uid", {
                    table: table,
                    value: existingModel,
                    uid: key,
                });
            }
        }
        else {
            switch (op){
                case "INSERT":
                    return db.query("INSERT INTO $table VALUES ($value)", {
                        value: value,
                        table: table,
                    });
                case "DELETE":
                    return db.query("DELETE FROM $table WHERE uid = $uid", {
                        uid: key,
                        table: table,
                    });
                case "SET":
                    setValueFromKeypath(existingModel, keypath, value);
                    return await db.query("UPDATE $table SET $value WHERE uid = $uid", {
                        table: table,
                        value: existingModel,
                        uid: key,
                    });
                case "UNSET":
                    unsetValueFromKeypath(existingModel, keypath);
                    return await db.query("UPDATE $table SET $value WHERE uid = $uid", {
                        table: table,
                        uid: key,
                        value: existingModel,
                    });
                default:
                    console.error(`Unknown OP: ${op}`);
                    break;
            }
        }
    }
}
const cc = new ControlCenter();
export default cc;
