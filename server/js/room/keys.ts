














const TYPES_INTO = new Set(["INPUT", "TEXTAREA", "SELECT"]);






export function typing(target: EventTarget | null): boolean {
	const el = target as { tagName?: string; isContentEditable?: boolean } | null;
	if (!el?.tagName) {
		return false;
	}

	return el.isContentEditable === true || TYPES_INTO.has(el.tagName);
}
