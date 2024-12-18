import Component from "~brixi/component";
interface ILink {
    label?: string;
    icon?: string;
    ariaLabel?: string;
    id: string;
}
export interface IBreadcrumbTrail {
    links: Array<ILink>;
}
export default class BreadcrumbTrail extends Component<IBreadcrumbTrail> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    private handleClick;
    private renderIcon;
    private renderLink;
    render(): void;
}
export {};
