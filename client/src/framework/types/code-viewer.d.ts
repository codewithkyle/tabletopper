import SuperComponent from "@codewithkyle/supercomponent";
type SourceCode = {
    ext: string;
    raw: string;
};
type CodeViewerData = {
    sourceCode: Array<SourceCode>;
    activeExt: string;
};
export default class CodeViewer extends SuperComponent<CodeViewerData> {
    private component;
    constructor(component: string);
    private fetchFiles;
    private switchSource;
    private copyToClipboard;
    render(): void;
}
export {};
