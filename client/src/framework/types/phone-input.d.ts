import { InputBase, IInputBase } from "../input-base";
interface IPhoneInput extends IInputBase {
    label: string;
    instructions: string;
    autocomplete: string;
    icon: string;
    placeholder: string;
    datalist: string[];
    autofocus: boolean;
    value: string;
}
export default class PhoneInput extends InputBase<IPhoneInput> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    validate(): boolean;
    /**
     * Formats phone number string (US)
     * @see https://stackoverflow.com/a/8358141
     * @license https://creativecommons.org/licenses/by-sa/4.0/
     */
    private formatPhoneNumber;
    private handleBlur;
    private handleFocus;
    private handleInput;
    private renderCopy;
    private renderIcon;
    private renderLabel;
    render(): void;
}
export {};
