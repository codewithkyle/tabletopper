import type { ButtonType } from "~brixi/components/buttons/button/button";
import Component from "~brixi/component";
export interface IGroupButton {
    buttons: Array<{
        label: string;
        type?: ButtonType;
        icon?: string;
        id: string;
    }>;
    active?: string;
}
export default class GroupButton extends Component<IGroupButton> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleClick;
    private renderIcon;
    private renderLabel;
    private renderButtons;
    render(): void;
}
