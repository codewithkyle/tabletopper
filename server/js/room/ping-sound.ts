// The noise a ping makes, and the one line of menu that turns it off.
//
// A PING IS "LOOK HERE" AND HALF THE TIME NOBODY IS LOOKING. That is the whole
// case for a sound: the rings are a second of attention aimed at somebody whose
// eyes are on a character sheet, a stat block, or the person talking. Without
// it the feature works only for the people who did not need it.
//
// IT IS SYNTHESISED RATHER THAN PLAYED FROM A FILE, which is the decision here
// worth defending. A file means a binary in the repository that cannot be read,
// diffed or reviewed; a route; a cache; and a failure mode -- the fetch that
// quietly does not land -- whose only symptom is silence, which is also exactly
// what a working mute looks like. Two oscillator settings and an envelope are
// four constants somebody can change by editing a line.
//
// THE CONTEXT IS MADE ON THE FIRST PING AND NOT AT MOUNT. An AudioContext
// created before any user gesture starts suspended, and a room whose table
// stays quiet all evening should not have taken an audio device for it. Every
// entry point is wrapped: a browser that refuses, a context that will not
// resume, and a viewer who has never clicked all end in silence rather than in
// an exception thrown out of the socket's fan-out.
//
// THE MUTE IS THIS BROWSER'S AND NOT THIS ACCOUNT'S, which is where it parts
// company with the blood. Blood is about content -- what a person can stand to
// look at for four hours -- and belongs to them wherever they sit down. A sound
// is about the room they are sitting IN: whether they have headphones on,
// whether they are in a voice call, whether somebody is asleep upstairs. That
// is a fact about a device and an evening, so it is localStorage, and it is
// reached from the menu rather than from the Ping tool's own pill -- because
// the moment somebody wants to mute this is the moment somebody ELSE is
// pinging, and a pill is only on screen while its own tool is chosen.
//
// NO CLASS NAME AND NO WORDING IS WRITTEN HERE. server/js is not a Tailwind
// source, and the two readings of the menu row are both rendered by room.go --
// see RoomMenuItem.AltLabel. What this writes is [hidden].

// KEY is namespaced the way window.ts's geometry is: one origin serves the
// whole app, and an unprefixed "mute" would be a name any other feature could
// reasonably want.
export const KEY = "tabletopper:ping-muted";

// The row this owns, by the id room.go renders onto it. See roomPingSoundID:
// the item deliberately carries no data-room-action, because nothing about
// muting a sound crosses the bundle boundary.
const ROW = "room-ping-sound";

// LOW and HIGH are C6 and G6, and the rise between them is what makes this a
// gesture rather than a beep. A single tone at this pitch is a notification
// from an operating system; two rising notes is somebody saying "here".
//
// THEY ARE HIGH ON PURPOSE. A ping has to cut through whoever is talking, and
// the band a voice does not occupy is above it.
const LOW = 1046.5;
const HIGH = 1568;

// NOTE is how long each of the two lasts and GAP is the breath between them.
//
// THE ENVELOPE IS WHAT MAKES IT TWO NOTES AND NOT ONE, and getting that wrong
// is why this used to be a single boop. The pitch was stepped halfway through a
// note that was ALREADY decaying exponentially from its peak -- so by the
// moment the second pitch arrived the amplitude was 2.5 percent of peak, 32 dB
// down, which is scheduled and inaudible. The frequencies were never the
// problem. So the level is held FLAT across both notes and released only at the
// end, and the two are separated by a dip to nothing rather than by a step in
// pitch alone: two tones that merely butt together read as one tone that
// changed its mind.
const NOTE = 0.055;
const GAP = 0.008;

// ATTACK is fast enough to sound like a tap rather than a swell, and RELEASE is
// the tail on the second note -- the only part of this that decays, because it
// is the only part with nothing after it.
const ATTACK = 0.004;
const RELEASE = 0.05;

// FLOOR is the nothing an exponential ramp is allowed to reach. It cannot be
// zero: a ramp to zero is silently ignored, and a ramp FROM zero is invalid.
const FLOOR = 0.0001;

// BLIP is the whole thing, derived rather than written down twice. About a sixth
// of a second, because a sound that outlasts the glance it asks for is a sound
// people turn off.
export const BLIP = NOTE * 2 + RELEASE;

// GAIN is a fifth, which is quiet. This is punctuation on a sentence somebody is
// saying out loud, and it loses every argument with the voice it interrupts.
//
// IT CAME DOWN WHEN THE ENVELOPE WAS FIXED. A quarter was chosen against a
// level that fell away immediately; held flat across two notes, the same number
// is a good deal louder to an ear than it was on paper.
const GAIN = 0.2;

// MIN_GAP is how close together two of these may sound, and it is exactly one
// blip long -- so a note is always finished before the next one starts.
//
// TWO OF THESE OVERLAPPING ARE NOT TWO SOUNDS, they are one dirtier sound at
// twice the level, and six people pinging at once should not add up to a chord.
// A ping inside the gap still DRAWS -- it is a real ping and the rings are the
// truth of it -- it just does not get its own note.
const MIN_GAP = BLIP * 1000;

// AudioRamp is the slice of AudioParam the sound is scheduled onto, named here
// rather than taken as an AudioParam so that the SHAPE of the sound can be
// tested without an audio device. render/pings.ts exports ringAt for the same
// reason, and the reason is the same in both places: what can be wrong here is
// arithmetic over time, and arithmetic over time is what a headless test checks.
export interface AudioRamp {
	setValueAtTime(value: number, startTime: number): unknown;
	linearRampToValueAtTime(value: number, endTime: number): unknown;
	exponentialRampToValueAtTime(value: number, endTime: number): unknown;
}

// PEAK is the level each note is struck at, exported so a test can ask whether
// the sound reaches it as many times as it has notes.
export const PEAK = GAIN;

// voice schedules the whole sound: two pitches on one oscillator, and the level
// that makes them two notes rather than one.
//
// TWO NOTES, ONE OSCILLATOR, AND THE GAIN IS WHAT SEPARATES THEM. Every ramp
// below is scheduled against the value at the event before it, so the order is
// the shape:
//
//   0 -> PEAK     the first note is struck
//   held         through it, so it is actually heard
//   -> FLOOR     the breath between the two
//   -> PEAK      the second note is struck, a fifth higher
//   held
//   -> FLOOR     the only decay in the whole sound
//
// LINEAR EVERYWHERE EXCEPT THE LAST ONE. A linear move to and from nothing is
// what a struck note does at these lengths, and an exponential ramp cannot
// start from a value of zero anyway; the final tail is exponential because that
// is the one place there is time to hear the difference.
export function voice(pitch: AudioRamp, level: AudioRamp, at: number): void {
	pitch.setValueAtTime(LOW, at);
	pitch.setValueAtTime(HIGH, at + NOTE);

	level.setValueAtTime(0, at);
	level.linearRampToValueAtTime(PEAK, at + ATTACK);
	level.setValueAtTime(PEAK, at + NOTE - GAP);
	level.linearRampToValueAtTime(FLOOR, at + NOTE);
	level.linearRampToValueAtTime(PEAK, at + NOTE + ATTACK);
	level.setValueAtTime(PEAK, at + NOTE * 2);
	level.exponentialRampToValueAtTime(FLOOR, at + BLIP);
}

// MuteStore is the slice of Storage this uses, named so a test can hand it one
// that throws. A private window throws on the first read, and a browser set to
// block site data throws on the write.
export interface MuteStore {
	getItem(key: string): string | null;
	setItem(key: string, value: string): void;
}

export interface Mute {
	muted(): boolean;

	// toggle flips it and answers the new state. A store that refuses the write
	// still flips for THIS session: somebody who asked for quiet gets quiet,
	// and the only thing they lose is that the next reload asks again.
	toggle(): boolean;
}

// newMute is the preference on its own, with no DOM and no audio in it, so the
// part that has rules can be tested and the part that makes a noise cannot be.
//
// DEFAULT ON, which is to say not muted. A ping's whole job is to reach
// somebody who is not looking, and a feature that ships silent ships switched
// off for everybody who never finds the menu.
export function newMute(store: MuteStore | null): Mute {
	let quiet = false;
	try {
		quiet = store?.getItem(KEY) === "1";
	} catch {
		quiet = false;
	}

	return {
		muted: () => quiet,

		toggle() {
			quiet = !quiet;
			try {
				store?.setItem(KEY, quiet ? "1" : "0");
			} catch {
				// See the note on toggle: the session is what matters.
			}

			return quiet;
		},
	};
}

export interface PingSound {
	// play makes the noise, unless it is muted or the last one is still
	// sounding. The CALLER decides whether a ping deserves one at all -- see
	// main.ts, which declines this viewer's own and any on a floor they are not
	// looking at.
	play(): void;

	stop(): void;
}

export function mountPingSound(): PingSound {
	const mute = newMute(storage());

	// The two readings of the menu row, in the order room.go renders them:
	// what the button DOES, then what it would do once it had. Only one is ever
	// on screen.
	const row = document.getElementById(ROW);
	const readings = row ? Array.from(row.querySelectorAll("span")) : [];

	let context: AudioContext | null = null;

	// last is when the previous blip sounded, and it starts BEFORE TIME rather
	// than at zero. Zero is a real reading -- performance.now() counts from the
	// moment the page started loading -- so a nought here says "one sounded at
	// page load" and swallows every ping in the first MIN_GAP milliseconds of
	// the room's life. Nothing at a table can ping that early, which is exactly
	// why it would never have been noticed.
	let last = Number.NEGATIVE_INFINITY;

	function paint(): void {
		readings.forEach((span, i) => {
			// The second reading is the one that means "turn it back on", so it
			// is on screen exactly when the sound is off.
			const undoes = i === 1;
			span.hidden = undoes !== mute.muted();
		});
	}

	function onClick(e: Event): void {
		if (!row || !(e.target instanceof Node) || !row.contains(e.target)) {
			return;
		}

		mute.toggle();
		paint();
	}

	// resumed answers a context that is ready, making one the first time it is
	// asked. A browser with no AudioContext at all, or one that refuses to
	// build one, answers null for ever after.
	function resumed(): AudioContext | null {
		if (context === null) {
			try {
				context = new AudioContext();
			} catch {
				return null;
			}
		}

		// A CONTEXT CAN BE SUSPENDED AT ANY POINT AND NOT ONLY AT THE START.
		// A tab in the background is the ordinary case, and coming back to the
		// tab is what resumes it -- so this is asked every time rather than
		// once. The promise is deliberately unawaited: a ping is worth a noise
		// NOW or not at all, and the frame after a resume settles is a frame
		// too late to be the sound of the thing that just happened.
		if (context.state === "suspended") {
			void context.resume().catch(() => {});

			// AND A VIEWER WHO HAS NOT TOUCHED THE PAGE CANNOT BE RESUMED AT
			// ALL, which is the case this arms for. Every browser refuses audio
			// until the page has been interacted with, and following a link
			// into a room is an interaction with the page somebody LEFT -- so a
			// player who joins and then sits watching the map is a player whose
			// context stays suspended however often it is asked. Their next
			// press on anything is what opens it, and from there every ping is
			// heard. Re-adding the same listener while one is pending does
			// nothing, so this does not stack up.
			document.addEventListener("pointerdown", wake, { once: true });
		}

		return context;
	}

	function wake(): void {
		void context?.resume().catch(() => {});
	}

	document.addEventListener("click", onClick);
	paint();

	return {
		play() {
			if (mute.muted()) {
				return;
			}

			const now = performance.now();
			if (now - last < MIN_GAP) {
				return;
			}

			const ctx = resumed();
			if (ctx === null || ctx.state !== "running") {
				return;
			}
			last = now;

			try {
				const at = ctx.currentTime;
				const osc = ctx.createOscillator();
				const gain = ctx.createGain();

				osc.type = "sine";
				voice(osc.frequency, gain.gain, at);

				osc.connect(gain).connect(ctx.destination);
				osc.start(at);
				osc.stop(at + BLIP);
			} catch {
				// An audio device that went away mid-session, which is a laptop
				// being unplugged from a dock. The table carries on.
			}
		},

		stop() {
			document.removeEventListener("click", onClick);
			document.removeEventListener("pointerdown", wake);
			void context?.close().catch(() => {});
			context = null;
		},
	};
}

// storage is localStorage or nothing, because reading the property itself
// throws in a browser set to block site data -- before any key is asked for.
function storage(): MuteStore | null {
	try {
		return window.localStorage;
	} catch {
		return null;
	}
}
