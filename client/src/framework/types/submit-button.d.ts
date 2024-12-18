import "~brixi/components/progress/spinner/spinner";
import Component from "~brixi/component";
import type { ButtonSize } from "../button/button";
export interface ISubmitButton {
    label: string;
    icon: string;
    size: ButtonSize;
    disabled: boolean;
    submittingLabel: string;
}
export default class SubmitButton extends Component<ISubmitButton> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleClick;
    private renderIcon;
    private renderLabel;
    render(): void;
}
