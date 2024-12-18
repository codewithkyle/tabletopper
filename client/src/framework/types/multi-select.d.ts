import { TemplateResult } from "lit-html";
import "~brixi/components/checkbox/checkbox";
import Component from "~brixi/component";
export type MultiSelectOption = {
    label: string;
    value: string | number;
    checked?: boolean;
    uid?: string;
};
export interface IMultiSelect {
    label: string;
    icon: string;
    instructions: string;
    options: Array<MultiSelectOption>;
    required: boolean;
    name: string;
    error: string;
    disabled: boolean;
    query: string;
    placeholder: string;
    search: "fuzzy" | "strict" | null;
    separator: string;
}
export default class MultiSelect extends Component<IMultiSelect> {
    private inputId;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    clearError(): void;
    setError(error: string): void;
    reset(): void;
    getName(): string;
    getValue(): any[];
    validate(): boolean;
    private hasOneCheck;
    private calcSelected;
    private filterOptions;
    private updateQuery;
    private debounceFilterInput;
    private handleFilterInput;
    private checkAllCallback;
    private checkboxCallback;
    renderCopy(): string | TemplateResult<2 | 1>;
    renderIcon(): string | TemplateResult<2 | 1>;
    renderLabel(): string | TemplateResult<2 | 1>;
    private renderSearch;
    render(): void;
}
