declare class Positions {
    window: {
        innerWidth: number;
        innerHeight: number;
        outterWidth: number;
        outterHeight: number;
    };
    constructor();
    private doResize;
    positionElement(el: HTMLElement, x: number, y: number): void;
    positionElementToElement(el: HTMLElement, target: HTMLElement, offset?: number): void;
}
declare const pos: Positions;
export default pos;
