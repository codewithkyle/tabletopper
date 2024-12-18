import Component from "~brixi/component";
export interface IStatusBadge {
    color: "grey" | "primary" | "success" | "warning" | "danger";
    label: string;
    dot: "right" | "left" | null;
    icon: string;
}
export default class StatusBadge extends Component<IStatusBadge> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    render(): void;
}
