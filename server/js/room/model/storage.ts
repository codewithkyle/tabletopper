const PREFIX = "tabletopper:";
export function store(key: string, value: unknown): void {
	try {
		localStorage.setItem(PREFIX + key, JSON.stringify(value));
	} catch {
	}
}
export function read<T>(key: string): T | null {
	try {
		const raw = localStorage.getItem(PREFIX + key);
		return raw === null ? null : (JSON.parse(raw) as T);
	} catch {
		return null;
	}
}
