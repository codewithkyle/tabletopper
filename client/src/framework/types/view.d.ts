import SuperComponent from "@codewithkyle/supercomponent";
type ViewData = {
    component: string;
    view: "demo" | "docs" | "code";
};
export default class View extends SuperComponent<ViewData> {
    private inboxId;
    constructor();
    private inbox;
    private load;
    private switchView;
    private renderContent;
    connected(): void;
    render(): void;
}
export {};
