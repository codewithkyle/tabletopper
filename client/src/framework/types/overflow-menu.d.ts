import Component from "~brixi/component";
export interface OverflowItem {
    label: string;
    id: string;
    icon?: string;
    danger?: boolean;
}
export interface IOverflowMenu {
    items: Array<OverflowItem>;
    uid: string;
    offset?: number;
    target: HTMLElement;
    callback: (id: string) => void;
}
export default class OverflowMenu extends Component<IOverflowMenu> {
    constructor(settings: IOverflowMenu);
    connected(): void;
    private handleItemClick;
    private renderItem;
    render(): void;
}
