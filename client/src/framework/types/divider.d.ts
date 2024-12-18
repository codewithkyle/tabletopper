import Component from "~brixi/component";
export type DividerColor = "primary" | "success" | "warning" | "danger" | "black" | "grey";
export interface IDivider {
    label: string;
    color: DividerColor;
    layout: "horizontal" | "vertical";
    type: "solid" | "dashed" | "dotted";
}
export default class Divider extends Component<IDivider> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    render(): void;
}
