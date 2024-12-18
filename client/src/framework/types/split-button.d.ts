import { OverflowItem } from "~brixi/components/overflow-menu/overflow-menu";
import Component from "~brixi/component";
import type { ButtonType } from "../button/button";
export interface ISplitButton {
    type: ButtonType;
    label: string;
    icon?: string;
    buttons: OverflowItem[];
    id: string;
}
export default class SplitButton extends Component<ISplitButton> {
    private uid;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private hideMenu;
    private handlePrimaryClick;
    private openMenu;
    private renderIcon;
    private renderLabel;
    private renderPrimaryButton;
    private renderMenuButtons;
    render(): void;
}
