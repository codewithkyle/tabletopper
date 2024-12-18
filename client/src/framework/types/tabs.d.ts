import "~brixi/components/buttons/button/button";
import Component from "~brixi/component";
export interface ITab {
    label: string;
    value: string | number;
    icon?: string;
    active?: boolean;
    index?: number;
}
export interface ITabs {
    tabs: Array<ITab>;
    active: number;
    sortable: boolean;
    expandable: boolean;
    shrinkable: boolean;
}
export default class Tabs extends Component<ITabs> {
    private firstRender;
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    getOrder(): Array<string | number>;
    private handleClick;
    callback(value: string | number, index: number): void;
    private sort;
    private addTab;
    resetIndexes(): void;
    private renderAddButton;
    render(): void;
}
