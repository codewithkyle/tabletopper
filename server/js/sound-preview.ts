import { FULL, newSpeaker } from "./room/sound.ts";
import { newPingSound } from "./room/ping-sound.ts";
import { newTurnSound } from "./room/turn-sound.ts";
interface Previewed {
	volume(percent: number): void;
	play(): void;
}
const speaker = newSpeaker();
const ping = newPingSound(speaker);
const turn = newTurnSound(speaker);
function slider(target: EventTarget | null): HTMLInputElement | null {
	if (!(target instanceof HTMLInputElement) || target.type !== "range") {
		return null;
	}
	return target.hasAttribute("data-sound-preview") ? target : null;
}
function chosen(input: HTMLInputElement): Previewed | null {
	switch (input.dataset.soundPreview) {
		case "ping":
			return ping;
		case "turn":
			return turn;
		default:
			return null;
	}
}
function level(input: HTMLInputElement): number {
	const value = Number.parseInt(input.value, 10);
	return Number.isFinite(value) ? value : FULL;
}
document.addEventListener("pointerdown", (e) => {
	if (slider(e.target) !== null) {
		speaker.running();
	}
});
document.addEventListener("change", (e) => {
	const input = slider(e.target);
	if (input === null) {
		return;
	}
	const sound = chosen(input);
	if (sound === null) {
		return;
	}
	sound.volume(level(input));
	sound.play();
});
