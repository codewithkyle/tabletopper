import "~brixi/components/progress/progress-indicator/progress-indicator";
import Component from "~brixi/component";
export interface IProgressBadge {
    label: string;
    total: number;
    color: "grey" | "primary" | "success" | "warning" | "danger";
}
export default class ProgressBadge extends Component<IProgressBadge> {
    private indicator;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    tick(): void;
    reset(): void;
    render(): void;
}
