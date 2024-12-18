import Component from "~brixi/component";
export interface ICheckbox {
    label: string;
    required: boolean;
    name: string;
    checked: boolean;
    error: string;
    disabled: boolean;
    type: "check" | "line";
    value: string | number;
}
export default class Checkbox extends Component<ICheckbox> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleChange;
    private handleKeydown;
    private handleKeyup;
    getName(): string;
    getValue(): string | number | null;
    reset(): void;
    clearError(): void;
    setError(error: string): void;
    validate(): boolean;
    private renderIcon;
    render(): void;
}
