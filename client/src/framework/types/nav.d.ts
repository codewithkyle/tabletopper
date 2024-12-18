import SuperComponent from "@codewithkyle/supercomponent";
type Link = {
    name: string;
    children: Array<Link>;
    slug: string;
};
type Navigation = Array<Link>;
type NavData = {
    navigation: Navigation;
    active: string;
};
export default class Nav extends SuperComponent<NavData> {
    constructor();
    private fetchNavigation;
    private navigate;
    private toggleGroup;
    private handleMenuClick;
    private renderLink;
    private renderLinkWithChildren;
    render(): void;
    connected(): void;
}
export {};
