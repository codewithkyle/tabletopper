import Component from "~brixi/component";
export interface ISuggestionChip {
    label: string;
    value: string | number;
}
export default class SuggestionChip extends Component<ISuggestionChip> {
    constructor();
    static get observedAttributes(): string[];
    connected(): void;
    private handleClick;
    private handleKeydown;
    private handleKeyup;
    render(): void;
}
