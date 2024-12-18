import Component from "~brixi/component";
import type { ButtonColor, ButtonKind, ButtonShape, ButtonSize } from "../button/button";
export interface IDownloadButton {
    label: string;
    icon: string;
    kind: ButtonKind;
    color: ButtonColor;
    shape: ButtonShape;
    size: ButtonSize;
    url: RequestInfo;
    options: RequestInit;
    downloadingLabel: string;
    workerURL: string;
}
export default class DownloadButton extends Component<IDownloadButton> {
    private indicator;
    private downloading;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private fetchData;
    private handleClick;
    private handleKeydown;
    private handleKeyup;
    private renderIcon;
    render(): void;
}
