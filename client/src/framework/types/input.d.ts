import { InputBase, IInputBase } from "../input-base";
export interface IInput extends IInputBase {
    label: string;
    instructions: string;
    autocomplete: string;
    autocapitalize: "off" | "on";
    icon: string;
    placeholder: string;
    maxlength: number;
    minlength: number;
    readOnly?: boolean;
    datalist: string[];
    autofocus: boolean;
    value: string;
}
export default class Input extends InputBase<IInput> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    validate(): boolean;
    private handleInput;
    private handleBlur;
    private handleFocus;
    private handleCopyClick;
    private renderCopy;
    private renderIcon;
    private renderReadOnlyIcon;
    private renderLabel;
    private renderDatalist;
    render(): void;
}
