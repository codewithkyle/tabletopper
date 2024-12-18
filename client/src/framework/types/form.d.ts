import Component from "~brixi/component";
export interface IForm {
}
export default class Form extends Component<IForm> {
    connected(): void;
    start(): void;
    stop(): void;
    reset(): void;
    serialize(): {};
    checkValidity(): boolean;
    fail(errors: {
        [name: string]: string;
    }): void;
    private handleReset;
}
