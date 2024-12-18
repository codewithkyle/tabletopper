import { IInputBase, InputBase } from "../input-base";
export interface IDateInput extends IInputBase {
    label: string;
    instructions: string;
    autocomplete: string;
    autocapitalize: "off" | "on";
    icon: string;
    placeholder: string;
    autofocus: boolean;
    value: string;
    dateFormat: string;
    displayFormat: string;
    enableTime: boolean;
    minDate: string;
    maxDate: string;
    mode: "multiple" | "single" | "range";
    disableCalendar: boolean;
    timeFormat: "24" | "12";
    prevValue: string | number;
}
export default class DateInput extends InputBase<IDateInput> {
    private firstRender;
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    private handleInput;
    private renderCopy;
    private renderIcon;
    private renderLabel;
    render(): void;
}
