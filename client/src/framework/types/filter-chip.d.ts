import Component from "~brixi/component";
export interface IFilterChip {
    label: string;
    value: string | number;
    checked: boolean;
}
export default class FilterChip extends Component<IFilterChip> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleClick;
    private handleKeydown;
    private handleKeyup;
    render(): void;
}
