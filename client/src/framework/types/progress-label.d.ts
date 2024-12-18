import Component from "~brixi/component";
import "../progress-indicator/progress-indicator";
export interface IProgressLabel {
    title: string;
    subtitle: string;
    total: number;
}
export default class ProgressLabel extends Component<IProgressLabel> {
    private indicator;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    tick(): void;
    reset(): void;
    setProgress(subtitle: string): void;
    render(): void;
}
