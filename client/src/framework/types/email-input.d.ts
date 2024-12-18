import { InputBase } from "../input-base";
import { IInput } from "../input/input";
export default class EmailInput extends InputBase<IInput> {
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
