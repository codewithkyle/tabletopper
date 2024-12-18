import Component from "~brixi/component";
export interface ISpinner {
    color: "primary" | "grey";
    size: number;
}
export default class Spinner extends Component<ISpinner> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    render(): void;
}
