import { InputBase, IInputBase } from "../input-base";
interface INumberInput extends IInputBase {
    label: string;
    instructions: string;
    icon: string;
    placeholder: string;
    autofocus: boolean;
    value: number | null;
    min: number;
    max: number;
    step: number;
}
export default class NumberInput extends InputBase<INumberInput> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    validate(): boolean;
    private handleInput;
    private handleBlur;
    private handleFocus;
    private renderCopy;
    private renderIcon;
    private renderLabel;
    render(): void;
}
export {};
