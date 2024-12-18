import Component from "~brixi/component";
export interface Step {
    label: string;
    description?: string;
    name: string;
}
export interface ISteps {
    steps: Array<Step>;
    activeStep: number;
    step: string;
    layout: "horizontal" | "vertical";
}
export default class Steps extends Component<ISteps> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleClick;
    private renderVerticalStep;
    private renderHorizontalStep;
    render(): void;
}
