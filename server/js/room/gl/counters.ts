export interface DrawCounts {
	calls: number;
	instances: number;
}
const counts: DrawCounts = { calls: 0, instances: 0 };
export function counted(instances: number): void {
	counts.calls++;
	counts.instances += instances;
}
export function drawCounts(): DrawCounts {
	return counts;
}
export function resetDrawCounts(): void {
	counts.calls = 0;
	counts.instances = 0;
}
