import SuperComponent from "@codewithkyle/supercomponent";
export interface ContextMenuItem {
    label: string;
    hotkey?: string;
    callback: Function;
}
export interface IContextMenu {
    items: ContextMenuItem[];
    x: number;
    y: number;
}
export interface ContextMenuSettings {
    items: ContextMenuItem[];
    x: number;
    y: number;
}
export default class ContextMenu extends SuperComponent<IContextMenu> {
    constructor(settings: ContextMenuSettings);
    connected(): void;
    private handleItemClick;
    private renderItem;
    render(): void;
}
