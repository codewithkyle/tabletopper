const windowMs = 1000;
export interface Counted {
	type: string;
	total: number;
	rate: number;
}
export interface Counts {
	add(type: string): void;
	sample(at: number): Counted[];
	reset(): void;
}
interface Tally {
	total: number;
	previous: number;
	rate: number;
}
export function newCounts(): Counts {
	const tallies = new Map<string, Tally>();
	let since = 0;
	let started = false;
	return {
		add(type) {
			const tally = tallies.get(type);
			if (tally) {
				tally.total++;
				return;
			}
			tallies.set(type, { total: 1, previous: 0, rate: 0 });
		},
		sample(at) {
			if (!started) {
				started = true;
				since = at;
			}
			const elapsed = at - since;
			const settle = elapsed >= windowMs;
			const out: Counted[] = [];
			for (const [type, tally] of tallies) {
				if (settle) {
					tally.rate = ((tally.total - tally.previous) * 1000) / elapsed;
					tally.previous = tally.total;
				}
				out.push({ type, total: tally.total, rate: tally.rate });
			}
			if (settle) {
				since = at;
			}
			out.sort((a, b) => (b.total - a.total) || (a.type < b.type ? -1 : 1));
			return out;
		},
		reset() {
			tallies.clear();
			started = false;
			since = 0;
		},
	};
}
