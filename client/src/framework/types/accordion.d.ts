import Component from "~brixi/component";
export interface AccordionSection {
    label: string;
    content: string;
}
export interface IAccordion {
    sections: Array<AccordionSection>;
}
export default class Accordion extends Component<IAccordion> {
    constructor();
    static get observedAttributes(): string[];
    connected(): void;
    private renderSection;
    render(): void;
}
