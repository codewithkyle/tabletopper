import Component from "~brixi/component";
export type LightswitchColor = "primary" | "success" | "warning" | "danger";
export interface ILightswitch {
    label: string;
    instructions: string;
    enabledLabel: string;
    disabledLabel: string;
    enabled: boolean;
    name: string;
    disabled: boolean;
    color: LightswitchColor;
    value: string | number;
    required: boolean;
}
export default class Lightswitch extends Component<ILightswitch> {
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
    private handleKeyup;
    private handleKeydown;
    private resize;
    render(): void;
}
