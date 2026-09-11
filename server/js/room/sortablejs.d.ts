declare module "sortablejs" {
	export interface SortableEvent {
		item: HTMLElement;
		oldIndex?: number;
		newIndex?: number;
	}
	export interface SortableOptions {
		draggable?: string;
		animation?: number;
		ghostClass?: string;
		chosenClass?: string;
		onStart?: (event: SortableEvent) => void;
		onEnd?: (event: SortableEvent) => void;
	}
	export default class Sortable {
		constructor(element: HTMLElement, options?: SortableOptions);
		destroy(): void;
	}
}
