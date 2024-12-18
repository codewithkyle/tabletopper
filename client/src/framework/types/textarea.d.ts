import { TemplateResult } from "lit-html";
import Component from "~brixi/component";
export interface ITextarea {
    label: string;
    name: string;
    instructions: string;
    error: string;
    required: boolean;
    autocomplete: string;
    placeholder: string;
    value: string;
    maxlength: number;
    minlength: number;
    disabled: boolean;
    readOnly: boolean;
    rows: number;
    autofocus: boolean;
}
export default class Textarea extends Component<ITextarea> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    clearError(): void;
    reset(): void;
    setError(error: string): void;
    validate(): boolean;
    getName(): string;
    getValue(): string;
    handleBlur: EventListener;
    handleFocus: EventListener;
    handleInput: EventListener;
    private handleCopyClick;
    renderCopy(): string | TemplateResult;
    renderLabel(): string | TemplateResult;
    private renderReadOnlyIcon;
    renderCounter(): string | TemplateResult;
    render(): void;
}
