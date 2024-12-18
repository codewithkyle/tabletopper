import SuperComponent from "@codewithkyle/supercomponent";
export default class Component<T> extends SuperComponent<T> {
    attributeChangedCallback(name: string, oldValue: string | null, newValue: string | null): void;
}
