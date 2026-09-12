import { ALERT } from "../../public/js/events.js";
import { read, store } from "./model/storage.ts";
import type { Event, Role, State } from "./protocol.ts";
const VOLUME_KEY = "music:volume";
const FULL = 100;
const DRIFT = 1.5;
const PERCENT = 100;
declare const htmx: {
	ajax(verb: string, path: string, context: { target: Element; swap: string }): Promise<void>;
};
export interface MusicPlayer {
	event(event: Event): void;
}
interface Started {
	id: string;
	url: string;
	contentType: string;
}
interface Problem {
	heading?: string;
	message?: string;
}
export function clock(seconds: number): string {
	if (!Number.isFinite(seconds) || seconds < 0) {
		return "0:00";
	}
	const whole = Math.floor(seconds);
	const minutes = Math.floor(whole / 60);
	return `${minutes}:${String(whole % 60).padStart(2, "0")}`;
}
export function clampVolume(value: number): number {
	if (!Number.isFinite(value)) {
		return FULL;
	}
	return Math.min(Math.max(Math.round(value), 0), FULL);
}
export function mountMusic(mount: HTMLElement, state: State, role: Role): MusicPlayer | null {
	const found = mount.querySelector("[data-music-player]");
	if (!(found instanceof HTMLAudioElement)) {
		return null;
	}
	const audio = found;
	const room = mount.dataset.room ?? "";
	let skew = 0;
	let volume = clampVolume(read<number>(VOLUME_KEY) ?? FULL);
	let blocked = false;
	let waking = false;
	audio.volume = volume / FULL;
	function span(): number {
		return Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : 0;
	}
	function target(): number {
		const music = state.music;
		const since = music.playing && music.since > 0 ? Date.now() + skew - music.since : 0;
		let seconds = (music.at + since) / 1000;
		const whole = span();
		if (whole > 0) {
			seconds = music.loop ? seconds % whole : Math.min(seconds, whole);
		}
		return Math.max(0, seconds);
	}
	function seek(seconds: number): void {
		try {
			audio.currentTime = seconds;
		} catch {
		}
	}
	function start(): void {
		void audio.play().then(() => setBlocked(false), () => setBlocked(true));
	}
	function setBlocked(value: boolean): void {
		blocked = value;
		if (blocked && !waking) {
			waking = true;
			document.addEventListener("pointerdown", woken, { once: true });
		}
		paintBlocked();
	}
	function woken(): void {
		waking = false;
		if (state.music.playing) {
			sync();
		}
	}
	function sync(): void {
		const music = state.music;
		if (music.trackId === null) {
			audio.pause();
			if (audio.hasAttribute("src")) {
				audio.removeAttribute("src");
				audio.load();
			}
			paint();
			return;
		}
		const src = `/rooms/${room}/music/audio?track=${music.trackId}`;
		if (audio.getAttribute("src") !== src) {
			audio.src = src;
			audio.load();
		}
		audio.loop = music.loop;
		const at = target();
		if (Math.abs(audio.currentTime - at) > DRIFT) {
			seek(at);
		}
		if (music.playing) {
			start();
		} else {
			audio.pause();
		}
		paint();
	}
	function paint(): void {
		const loaded = state.music.trackId !== null;
		const whole = span();
		const at = loaded ? audio.currentTime : 0;
		const bar = document.querySelector("[data-music-bar]");
		if (bar instanceof HTMLProgressElement) {
			bar.value = whole > 0 ? Math.round((at / whole) * bar.max) : 0;
		}
		write("[data-music-elapsed]", clock(at));
		write("[data-music-duration]", whole > 0 ? clock(whole) : "--:--");
		const slider = document.querySelector("[data-music-volume]");
		if (slider instanceof HTMLInputElement && slider.value !== String(volume)) {
			slider.value = String(volume);
		}
		write("[data-music-volume-value]", `${volume}%`);
		paintBlocked();
	}
	function paintBlocked(): void {
		const button = document.querySelector("[data-music-unblock]");
		if (button instanceof HTMLElement) {
			button.hidden = !blocked;
		}
	}
	function onVolume(e: globalThis.Event): void {
		const slider = e.target;
		if (!(slider instanceof HTMLInputElement) || !slider.hasAttribute("data-music-volume")) {
			return;
		}
		volume = clampVolume(Number.parseInt(slider.value, 10));
		audio.volume = volume / FULL;
		store(VOLUME_KEY, volume);
		write("[data-music-volume-value]", `${volume}%`);
	}
	function onClick(e: globalThis.Event): void {
		if (e.target instanceof Element && e.target.closest("[data-music-unblock]")) {
			sync();
		}
	}
	function onPicked(e: globalThis.Event): void {
		const picker = e.target;
		if (!(picker instanceof HTMLInputElement) || !picker.hasAttribute("data-music-file")) {
			return;
		}
		const file = picker.files?.[0];
		picker.value = "";
		if (file) {
			void upload(file);
		}
	}
	function onSwap(e: globalThis.Event): void {
		const swapped = e.target;
		if (!(swapped instanceof Element)) {
			return;
		}
		if (swapped.matches("[data-music-bar]") || swapped.querySelector("[data-music-bar]")) {
			paint();
		}
	}
	async function upload(file: File): Promise<void> {
		lock(true);
		showUpload(0);
		let started: Started;
		try {
			const response = await fetch("/assets/music", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ name: file.name, size: file.size }),
			});
			if (!response.ok) {
				const problem = await response.json().catch(() => null) as Problem | null;
				reset();
				alarm(
					problem?.heading ?? "Upload Failed",
					problem?.message ?? "The upload could not be started. Try again in a moment.",
				);
				return;
			}
			started = await response.json() as Started;
		} catch {
			reset();
			alarm("Upload Failed", "The upload could not be started. Check your connection and try again.");
			return;
		}
		try {
			await put(started.url, started.contentType, file);
		} catch {
			void fetch(`/assets/music/${started.id}`, { method: "DELETE" }).catch(() => {});
			reset();
			alarm("Upload Failed", "The track did not finish uploading. Try again.");
			return;
		}
		try {
			await confirm(started.id);
		} finally {
			reset();
			relist();
		}
	}
	function put(url: string, contentType: string, file: File): Promise<void> {
		return new Promise((resolve, reject) => {
			const request = new XMLHttpRequest();
			request.open("PUT", url);
			request.setRequestHeader("Content-Type", contentType);
			request.upload.addEventListener("progress", (e) => {
				if (e.lengthComputable) {
					showUpload(e.loaded / e.total);
				}
			});
			request.addEventListener("load", () => {
				if (request.status >= 200 && request.status < 300) {
					resolve();
					return;
				}
				reject(new Error(`the bucket answered ${request.status}`));
			});
			request.addEventListener("error", () => reject(new Error("the upload failed")));
			request.addEventListener("abort", () => reject(new Error("the upload was cancelled")));
			request.send(file);
		});
	}
	async function confirm(id: string): Promise<void> {
		const path = `/assets/music/${id}/confirm`;
		if (typeof htmx === "undefined") {
			await fetch(path, { method: "POST" });
			return;
		}
		const search = document.querySelector("[data-music-search]");
		const target = search instanceof Element ? search : document.body;
		await htmx.ajax("POST", path, { target, swap: "none" });
	}
	function relist(): void {
		const search = document.querySelector("[data-music-search]");
		if (search instanceof HTMLInputElement) {
			search.dispatchEvent(new CustomEvent("search"));
		}
	}
	function lock(uploading: boolean): void {
		const picker = document.querySelector("[data-music-file]");
		if (picker instanceof HTMLInputElement) {
			picker.disabled = uploading;
		}
		const label = document.querySelector("[data-music-upload]");
		if (!(label instanceof HTMLElement)) {
			return;
		}
		if (uploading) {
			label.setAttribute("aria-disabled", "true");
			return;
		}
		label.removeAttribute("aria-disabled");
	}
	function showUpload(fraction: number): void {
		const row = document.querySelector("[data-music-uploading]");
		if (row instanceof HTMLElement) {
			row.hidden = false;
		}
		const whole = Math.round(fraction * PERCENT);
		const bar = document.querySelector("[data-music-upload-bar]");
		if (bar instanceof HTMLProgressElement) {
			bar.value = whole;
		}
		write("[data-music-percent]", `${whole}%`);
	}
	function reset(): void {
		const row = document.querySelector("[data-music-uploading]");
		if (row instanceof HTMLElement) {
			row.hidden = true;
		}
		const bar = document.querySelector("[data-music-upload-bar]");
		if (bar instanceof HTMLProgressElement) {
			bar.value = 0;
		}
		write("[data-music-percent]", "");
		lock(false);
	}
	audio.addEventListener("loadedmetadata", () => sync());
	audio.addEventListener("timeupdate", () => paint());
	audio.addEventListener("play", () => paint());
	audio.addEventListener("pause", () => paint());
	audio.addEventListener("ended", () => {
		const music = state.music;
		if (role !== "gm" || music.trackId === null || music.loop || !music.playing) {
			return;
		}
		void fetch(`/rooms/${room}/music/ended`, {
			method: "POST",
			headers: { "Content-Type": "application/x-www-form-urlencoded" },
			body: `track=${encodeURIComponent(music.trackId)}`,
		}).catch(() => {});
	});
	document.addEventListener("input", onVolume);
	document.addEventListener("click", onClick);
	document.addEventListener("change", onPicked);
	document.addEventListener("htmx:after:swap", onSwap);
	return {
		event(event) {
			if (event.type === "snapshot") {
				skew = event.now - Date.now();
				sync();
				return;
			}
			if (event.type === "music.updated") {
				sync();
			}
		},
	};
}
function write(selector: string, value: string): void {
	const el = document.querySelector(selector);
	if (el instanceof HTMLElement && el.textContent !== value) {
		el.textContent = value;
	}
}
function alarm(heading: string, message: string): void {
	window.dispatchEvent(new CustomEvent(ALERT, { detail: { heading, message } }));
}
