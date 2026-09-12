import type { Outgoing } from "../socket.ts";
import type { Tool } from "../render/input.ts";
export interface PingDeps {
	viewed: () => string;
	send: (command: Outgoing) => void;
}
export function createPing(deps: PingDeps): Tool {
	return {
		press(map) {
			deps.send({
				type: "ping",
				layer: deps.viewed(),
				x: Math.round(map.x),
				y: Math.round(map.y),
			});
			return true;
		},
		drag() {},
		release() {},
		cancel() {},
		secondary: () => false,
		hover() {},
		key: () => false,
		abandon: () => false,
		active: () => false,
		contribute() {},
	};
}
