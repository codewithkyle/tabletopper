import Sortable from "sortablejs";
const GHOST = "sort-ghost";
const CHOSEN = "sort-chosen";
const SETTLE = 600;
const mounted = new Map<HTMLElement, Sortable>();
const waiting = new Map<HTMLElement, ReturnType<typeof setTimeout>>();
function rowIDs(list: Element): string[] {
	const ids: string[] = [];
	for (const row of list.children) {
		const id = row.getAttribute("data-sort-row");
		if (id) {
			ids.push(id);
		}
	}
	return ids;
}
function save(list: HTMLElement): void {
	clearTimeout(waiting.get(list));
	waiting.delete(list);
	const button = document.getElementById(list.dataset.sortList ?? "");
	if (!button) {
		return;
	}
	button.setAttribute("hx-vals", JSON.stringify({ ids: rowIDs(list).join(",") }));
	button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
}
function saveSoon(list: HTMLElement): void {
	clearTimeout(waiting.get(list));
	waiting.set(list, setTimeout(() => save(list), SETTLE));
}
function mount(): void {
	for (const [list, sortable] of mounted) {
		if (!list.isConnected) {
			sortable.destroy();
			mounted.delete(list);
		}
	}
	for (const found of document.querySelectorAll("[data-sort-list]")) {
		if (!(found instanceof HTMLElement) || mounted.has(found)) {
			continue;
		}
		const list = found;
		mounted.set(
			list,
			new Sortable(list, {
				draggable: "[data-sort-row]",
				handle: "[data-sort-handle]",
				animation: 150,
				ghostClass: GHOST,
				chosenClass: CHOSEN,
				onEnd(event) {
					if (event.oldIndex !== event.newIndex) {
						save(list);
					}
				},
			}),
		);
	}
}
function onKeyDown(event: KeyboardEvent): void {
	if (event.key !== "ArrowUp" && event.key !== "ArrowDown") {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) {
		return;
	}
	const handle = event.target instanceof HTMLElement ? event.target.closest("[data-sort-handle]") : null;
	const row = handle?.closest("[data-sort-row]");
	const list = row?.parentElement;
	if (!(handle instanceof HTMLElement) || !row || !list?.hasAttribute("data-sort-list")) {
		return;
	}
	event.preventDefault();
	const sibling = event.key === "ArrowUp" ? row.previousElementSibling : row.nextElementSibling;
	if (!sibling?.hasAttribute("data-sort-row")) {
		return;
	}
	if (event.key === "ArrowUp") {
		sibling.before(row);
	} else {
		sibling.after(row);
	}
	handle.focus();
	saveSoon(list);
}
document.addEventListener("keydown", onKeyDown);
document.addEventListener("htmx:after:swap", mount);
mount();
