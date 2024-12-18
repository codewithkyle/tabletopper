import Component from "~brixi/component";
export interface IAssistChip {
    label: string;
    icon: string;
}
export default class AssistChip extends Component<IAssistChip> {
    constructor();
    static get observedAttributes(): string[];
    connected(): void;
    private handleKeydown;
    private handleKeyup;
    private renderIcon;
    render(): void;
}
