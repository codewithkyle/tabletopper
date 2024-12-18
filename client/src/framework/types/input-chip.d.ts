import Component from "~brixi/component";
export interface IInputChip {
    label: string;
    value: string | number;
}
export default class InputChip extends Component<IInputChip> {
    constructor();
    static get observedAttributes(): string[];
    connected(): void;
    private handleClick;
    private handleKeydown;
    private handleKeyup;
    render(): void;
}
