import { IInputBase, InputBase } from "../input-base";
import "~brixi/utils/strings";
export interface IColorInput extends IInputBase {
    value: string;
    label: string;
    readOnly: boolean;
}
export default class ColorInput extends InputBase<IColorInput> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    validate(): boolean;
    private handleInput;
    render(): void;
}
