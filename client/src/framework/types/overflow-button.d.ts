import { OverflowItem } from "~brixi/components/overflow-menu/overflow-menu";
import Component from "~brixi/component";
import type { ButtonColor, ButtonKind, ButtonShape, ButtonSize } from "../button/button";
export interface IOverflowButton {
    icon: string;
    iconPosition: "left" | "right" | "center";
    kind: ButtonKind;
    color: ButtonColor;
    shape: ButtonShape;
    size: ButtonSize;
    disabled: boolean;
    items: Array<OverflowItem>;
}
export default class OverflowButton extends Component<IOverflowButton> {
    private uid;
    constructor();
    static get observedAttributes(): string[];
    connected(): void;
    private handleClick;
    render(): void;
}
