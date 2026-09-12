const EDITORS = "[data-journal-editor-src]";
const editors = new Map<HTMLElement, () => void>();
const idle = (): void => {};
export function mountJournalEditors(): void {
	document.addEventListener("htmx:after:settle", () => {
		sweep();
		for (const root of document.querySelectorAll(EDITORS)) {
			if (root instanceof HTMLElement && !editors.has(root)) {
				void open(root);
			}
		}
	});
}
function sweep(): void {
	for (const [root, stop] of editors) {
		if (!root.isConnected) {
			stop();
			editors.delete(root);
		}
	}
}
async function open(root: HTMLElement): Promise<void> {
	const src = root.dataset.journalEditorSrc;
	if (!src) {
		return;
	}
	editors.set(root, idle);
	try {
		const loaded = await import(src);
		if (!root.isConnected) {
			editors.delete(root);
			return;
		}
		editors.set(root, loaded.start(root));
	} catch (err) {
		console.error("the journal editor could not be loaded", err);
		editors.delete(root);
	}
}
