import Component from "~brixi/component";
export interface IRadio {
    label: string;
    required: boolean;
    name: string;
    checked: boolean;
    disabled: boolean;
    value: string | number;
}
export default class Radio extends Component<IRadio> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    getName(): string;
    getValue(): string | number | null;
    reset(): void;
    clearError(): void;
    setError(error: string): void;
    validate(): boolean;
    private handleChange;
    private handleKeydown;
    private handleKeyup;
    render(): void;
}
