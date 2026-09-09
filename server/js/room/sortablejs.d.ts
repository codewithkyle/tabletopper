// The shape of SortableJS that this app uses, and nothing else.
//
// THE PACKAGE SHIPS NO TYPES and @types/sortablejs describes a surface an order
// of magnitude wider than the four options and one method below. A hand-written
// declaration of what is actually called is a file somebody can read in ten
// seconds, and it fails the build the day an option is misspelled -- which is
// the whole of what a type was going to buy here.
declare module "sortablejs" {
	export interface SortableEvent {
		item: HTMLElement;
		oldIndex?: number;
		newIndex?: number;
	}

	export interface SortableOptions {
		// draggable is the selector for what may be picked up, so that the
		// hidden buttons and the round counter inside the strip cannot be.
		draggable?: string;

		animation?: number;

		// ghostClass and chosenClass are OURS. They are options rather than
		// defaults so that nothing is written into the DOM that we did not
		// name; both are defined in server/css/app.css, because server/js is
		// not a Tailwind source and a class named in a script is never emitted.
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
