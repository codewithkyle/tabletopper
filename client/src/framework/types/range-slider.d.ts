import { InputBase, IInputBase } from "../input-base";
export interface IRangeSlider extends IInputBase {
    label: string;
    instructions: string;
    icon: string;
    readOnly: boolean;
    autofocus: boolean;
    min: number;
    max: number;
    step: number;
    manual: boolean;
    value: number;
    minIcon: string;
    maxIcon: string;
}
export default class RangeSlider extends InputBase<IRangeSlider> {
    private fillPercentage;
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    private handleChange;
    private handleInput;
    private handleBlur;
    private handleFocus;
    private handleIconClick;
    reset(): void;
    validate(): boolean;
    private renderCopy;
    private renderLabel;
    private renderManualInput;
    private renderFill;
    private renderIcon;
    render(): void;
}
