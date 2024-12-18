import Component from "~brixi/component";
export type ItemStyle = "disc" | "circle" | "decimal" | "leading-zero" | "square" | "custom";
export type ListType = "ordered" | "unordered";
export interface List {
    type: ListType;
    style?: ItemStyle;
    items: Array<string>;
    sub?: List;
    icon?: string;
}
export interface IGenericList {
    list: List;
}
export default class GenericList extends Component<IGenericList> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private renderStyleType;
    private renderItem;
    private renderList;
    render(): void;
}
