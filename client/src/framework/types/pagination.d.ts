import "~brixi/components/buttons/button/button";
import Component from "~brixi/component";
export interface IPagination {
    totalPages: number;
    activePage: number;
}
export default class Pagination extends Component<IPagination> {
    constructor();
    static get observedAttributes(): string[];
    connected(): Promise<void>;
    back(): void;
    forward(): void;
    jumpToPage(pageNumber: number): void;
    private handleBack;
    private handleForward;
    private processPageChange;
    private calcVisiblePageNumbers;
    render(): void;
}
