import SuperComponent from "@codewithkyle/supercomponent";
type DocViewerData = {
    html: string;
};
export default class DocViewer extends SuperComponent<DocViewerData> {
    private component;
    constructor(component: string);
    private fetchDoc;
    render(): void;
}
export {};
