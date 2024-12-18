import "~brixi/components/radio/radio";
import Component from "~brixi/component";
import type { IRadio } from "~brixi/components/radio/radio";
export interface IRadioGroup {
    options: Array<IRadio>;
    instructions: string;
    disabled: boolean;
    label: string;
    name: string;
    required: boolean;
}
export default class RadioGroup extends Component<IRadioGroup> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    getName(): string;
    getValue(): string | number | null;
    reset(): void;
    clearError(): void;
    setError(error: string): void;
    validate(): boolean;
    render(): void;
}
