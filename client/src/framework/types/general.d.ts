/**
 * A generic no operation (noop) function.
 */
export declare function noop(): void;
/**
 * Debounce a function callback.
 */
export declare const debounce: (callback: Function, wait: number) => Function;
/**
 * Maps a `DOMStringMap` onto an object.
 * @see https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/dataset
 */
export declare function parseDataset<T>(dataset: DOMStringMap, model: T): T;
