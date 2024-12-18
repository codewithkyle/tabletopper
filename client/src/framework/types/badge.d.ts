import Component from "~brixi/component";
export interface IBadge {
    value: number;
    offsetX: number;
    offsetY: number;
}
export default class Badge extends Component<IBadge> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    render(): void;
}
