import "~brixi/components/buttons/button/button";
import type { IButton } from "~brixi/components/buttons/button/button";
import Component from "~brixi/component";
interface Button extends IButton {
    id: string;
}
export interface IToggleButton {
    state: string;
    states: Array<string>;
    buttons: {
        [state: string]: Button;
    };
    instructions: string;
    index: number;
}
export default class ToggleButton extends Component<IToggleButton> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleAction;
    private handleClick;
    private renderButton;
    private renderInstructions;
    render(): void;
}
export {};
