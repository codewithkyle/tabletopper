import { InputBase, IInputBase } from "../input-base";
interface IPasswordInput extends IInputBase {
    label: string;
    instructions: string;
    autocomplete: string;
    icon: string;
    placeholder: string;
    maxlength: number;
    minlength: number;
    autofocus: boolean;
    value: string;
    type: "text" | "password";
}
export default class PasswordInput extends InputBase<IPasswordInput> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    validate(): boolean;
    private toggleVisibility;
    private handleInput;
    private handleBlur;
    private handleFocus;
    private renderCopy;
    private renderIcon;
    private renderLabel;
    private renderEyeIcon;
    render(): void;
}
export {};
