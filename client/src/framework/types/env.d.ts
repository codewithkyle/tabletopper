export type NetworkType = "4g" | "3g" | "2g" | "slow-2g";
export type DOMState = "loading" | "idling" | "booting";
export type Browser = "chrome" | "safari" | "edge" | "chromium-edge" | "ie" | "firefox" | "unknown" | "opera";
declare class Environment {
    connection: NetworkType;
    cpu: number;
    memory: number | null;
    domState: DOMState;
    dataSaver: boolean;
    browser: Browser;
    private tickets;
    constructor();
    boot(): void;
    private handleNetworkChange;
    /**
     * Attempts to set the DOM to the `idling` state. The DOM will only idle when all `startLoading()` methods have been resolved.
     * @param ticket - the `string` the was provided by the `startLoading()` method.
     */
    stopLoading(ticket: string): void;
    /**
     * Sets the DOM to the `soft-loading` state.
     * @returns a ticket `string` that is required to stop the loading state.
     */
    startLoading(): string;
    /**
     * Sets the DOMs state attribute.
     * DO NOT USE THIS METHOD. DO NOT MANUALLY SET THE DOMs STATE.
     * @param newState - the new state of the document element
     */
    private setDOMState;
    /**
     * Checks if the provided connection is greater than or equal to the current conneciton.
     * @param requiredConnection - network connection string
     */
    checkConnection(requiredConnection: any): boolean;
    private setBrowser;
    /**
     * Binds the custom element to the class.
     * @deprecated use `bind()` instead.
     */
    mount(tagName: string, constructor: CustomElementConstructor): void;
    /**
     * Registers a Web Component by binding the Custom Element's tag name to the provided class.
     */
    bind(tagName: string, constructor: CustomElementConstructor): void;
    css(files: string | string[]): Promise<void>;
}
declare const env: Environment;
export { env as default };
