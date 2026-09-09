// Which keys are the page's and which belong to whatever has the caret.
//
// A KEY PRESSED IN A FIELD IS NOT A KEY PRESSED ON THE TABLE. Every key the
// room listens for is heard on the DOCUMENT, because the canvas is not what has
// focus while a hand is on the keyboard -- so a GM typing a goblin's new name
// into a pawn window is pressing Delete to rub out a letter and the space bar to
// put a space between two words, and both of those would otherwise reach the
// table.
//
// IT IS ITS OWN MODULE BECAUSE TWO MODULES ASK. pawns.ts hears Escape and
// Delete; tools.ts hears the space bar. A copy in each is a copy that gets
// fixed in one of them.

// The three elements text goes into. A contenteditable is asked about
// separately below, because it is a property rather than a tag.
const TYPES_INTO = new Set(["INPUT", "TEXTAREA", "SELECT"]);

// typing is whether an event came from somewhere text goes.
//
// THE TARGET IS DUCK-TYPED RATHER THAN CHECKED WITH instanceof, so that the
// tests can hand it a plain object: node has no HTMLElement for one to be an
// instance of.
export function typing(target: EventTarget | null): boolean {
	const el = target as { tagName?: string; isContentEditable?: boolean } | null;
	if (!el?.tagName) {
		return false;
	}

	return el.isContentEditable === true || TYPES_INTO.has(el.tagName);
}
