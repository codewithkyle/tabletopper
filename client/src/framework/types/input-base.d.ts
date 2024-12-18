import Component from "~brixi/component";
export interface IInputBase {
    name: string;
    error: string;
    required: boolean;
    value: any;
    disabled: boolean;
}
export declare class InputBase<T> extends Component<T> {
    constructor();
    connected(): Promise<void>;
    reset(): void;
    clearError(): void;
    setError(error: string): void;
    validate(): boolean;
    getName(): string;
    getValue(): any;
}
