import { TemplateResult } from "lit-html";
import Component from "~brixi/component";
export type SelectOption = {
    label: string;
    value: string;
};
export interface ISelect {
    label: string;
    icon: string | HTMLElement;
    instructions: string;
    options: Array<SelectOption>;
    required: boolean;
    name: string;
    error: string;
    value: any;
    disabled: boolean;
    autofocus: boolean;
}
export default class Select extends Component<ISelect> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    renderCopy(): string | TemplateResult;
    renderIcon(): string | TemplateResult;
    clearError(): void;
    reset(): void;
    setError(error: string): void;
    validate(): boolean;
    private handleChange;
    getName(): string;
    getValue(): any;
    handleBlur: EventListener;
    private handleFocus;
    renderLabel(): string | TemplateResult;
    render(): void;
}
