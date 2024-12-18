import "~brixi/components/buttons/button/button";
import Component from "~brixi/component";
export interface ActionItem {
    label: string;
    id: string;
}
export interface IAlert {
    type: "warning" | "info" | "danger" | "success";
    heading: string;
    description: string;
    list: Array<string>;
    closeable: boolean;
    actions: Array<ActionItem>;
}
export default class Alert extends Component<IAlert> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private renderIcon;
    private handleClose;
    private handleActionClick;
    private renderCloseButton;
    private renderList;
    private renderActions;
    render(): void;
}
