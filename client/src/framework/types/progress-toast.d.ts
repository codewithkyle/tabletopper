import Component from "~brixi/component";
import "../progress-indicator/progress-indicator";
export interface IProgressToast {
    title: string;
    subtitle: string;
    total: number;
}
export default class ProgressToast extends Component<IProgressToast> {
    private indicator;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    tick(amount?: number): void;
    reset(): void;
    setProgress(subtitle: string): void;
    private finishedCallback;
    render(): void;
}
