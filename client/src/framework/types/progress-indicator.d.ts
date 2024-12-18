import Component from "~brixi/component";
export interface IProgressIndicator {
    size: number;
    tick: number;
    total: number;
    color: "grey" | "primary" | "success" | "warning" | "danger" | "white";
}
export default class ProgressIndicator extends Component<IProgressIndicator> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    /**
     * Resets the `tick` value to `0`.
     */
    reset(): void;
    tick(amount?: number): void;
    /**
     * Sets the total and resets the `tick` value to `0`.
     */
    setTotal(total: number): void;
    private calcDashOffset;
    render(): void;
}
