import type { ICheckbox } from "~brixi/components/checkbox/checkbox";
import "~brixi/components/checkbox/checkbox";
import Component from "~brixi/component";
export interface ICheckboxGroup {
    options: Array<ICheckbox>;
    instructions: string;
    disabled: boolean;
    label: string;
    name: string;
}
export default class CheckboxGroup extends Component<ICheckboxGroup> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    getName(): string;
    getValue(): Array<string | number>;
    reset(): void;
    clearError(): void;
    setError(error: string): void;
    render(): void;
}
