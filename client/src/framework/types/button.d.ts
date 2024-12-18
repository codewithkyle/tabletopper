import Component from "~brixi/component";
export type ButtonKind = "solid" | "outline" | "text";
export type ButtonColor = "primary" | "danger" | "grey" | "success" | "warning" | "white";
export type ButtonShape = "pill" | "round" | "sharp" | "default";
export type ButtonSize = "default" | "slim" | "large";
export type ButtonType = "submit" | "button" | "reset";
export interface IButton {
    label: string;
    icon: string;
    iconPosition: "left" | "right" | "center";
    kind: ButtonKind;
    color: ButtonColor;
    shape: ButtonShape;
    size: ButtonSize;
    disabled: boolean;
    type: ButtonType;
}
export default class Button extends Component<IButton> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private renderIcon;
    private renderLabel;
    private dispatchClick;
    private handleClick;
    private handleKeydown;
    private handleKeyup;
    render(): void;
}
