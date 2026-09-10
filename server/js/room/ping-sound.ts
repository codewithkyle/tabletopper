// The noise a ping makes, and how loud this viewer wants it.
//
// A PING IS "LOOK HERE" AND HALF THE TIME NOBODY IS LOOKING. That is the whole
// case for a sound: the rings are a second of attention aimed at somebody whose
// eyes are on a character sheet, a stat block, or the person talking. Without it
// the feature works only for the people who did not need it.
//
// IT IS SYNTHESISED RATHER THAN PLAYED FROM A FILE, which is the decision here
// worth defending. A file means a binary in the repository that cannot be read,
// diffed or reviewed; a route; a cache; and a failure mode -- the fetch that
// quietly does not land -- whose only symptom is silence, which is also exactly
// what a volume at zero looks like. Two pitches and an envelope are six
// constants somebody can change by editing a line.
//
// THE CONTEXT IS MADE ON THE FIRST PING AND NOT AT MOUNT. An AudioContext
// created before any user gesture starts suspended, and a room whose table stays
// quiet all evening should not have taken an audio device for it. Every entry
// point is wrapped: a browser that refuses, a context that will not resume, and
// a viewer who has never clicked all end in silence rather than in an exception
// thrown out of the socket's fan-out.
//
// HOW LOUD IS AN ACCOUNT SETTING AND NOT THIS FILE'S. It arrives as a percentage
// -- from the attribute the room page renders, and again from the settings dialog
// whenever it is saved -- and NOTHING IS STORED HERE. There is deliberately no
// second control in the room: a mute in the Tabletop menu was built first and
// removed, because a slider whose bottom stop is silence already answers "at
// all" as well as "how loud", and two controls over one setting are two things
// that can disagree. See PingVolume in internal/prefs.

// LOW and HIGH are C6 and G6, and the rise between them is what makes this a
// gesture rather than a beep. A single tone at this pitch is a notification from
// an operating system; two rising notes is somebody saying "here".
//
// THEY ARE HIGH ON PURPOSE. A ping has to cut through whoever is talking, and
// the band a voice does not occupy is above it.
const LOW = 1046.5;
const HIGH = 1568;

// NOTE is how long each of the two lasts and GAP is the breath between them.
//
// THE ENVELOPE IS WHAT MAKES IT TWO NOTES AND NOT ONE, and getting that wrong is
// why this used to be a single boop. The pitch was stepped halfway through a note
// that was ALREADY decaying exponentially from its peak -- so by the moment the
// second pitch arrived the amplitude was 2.5 percent of peak, 32 dB down, which
// is scheduled and inaudible. The frequencies were never the problem. So the
// level is held FLAT across both notes and released only at the end, and the two
// are separated by a dip to nothing rather than by a step in pitch alone: two
// tones that merely butt together read as one tone that changed its mind.
const NOTE = 0.055;
const GAP = 0.008;

// ATTACK is fast enough to sound like a tap rather than a swell, and RELEASE is
// the tail on the second note -- the only part of this that decays, because it is
// the only part with nothing after it.
const ATTACK = 0.004;
const RELEASE = 0.05;

// FLOOR is the nothing an exponential ramp is allowed to reach. It cannot be
// zero: a ramp to zero is silently ignored, and a ramp FROM zero is invalid.
const FLOOR = 0.0001;

// BLIP is the whole thing, derived rather than written down twice. About a sixth
// of a second, because a sound that outlasts the glance it asks for is a sound
// people turn off.
export const BLIP = NOTE * 2 + RELEASE;

// PEAK is the level each note is struck at with the volume all the way up, and a
// fifth is quiet. This is punctuation on a sentence somebody is saying out loud,
// and it loses every argument with the voice it interrupts.
export const PEAK = 0.2;

// FULL is the volume percentage that means PEAK, and it is what an ABSENT or
// unreadable setting falls back to.
//
// FULL RATHER THAN SILENT, and it matters which way round: a page served by a
// build that does not send the setting yet, or a value somebody put in devtools,
// must be a page where pings still work. A feature that fails quiet is a feature
// nobody reports as broken.
export const FULL = 100;

// MIN_GAP is how close together two of these may sound, and it is exactly one
// blip long -- so a note is always finished before the next one starts.
//
// TWO OF THESE OVERLAPPING ARE NOT TWO SOUNDS, they are one dirtier sound at
// twice the level, and six people pinging at once should not add up to a chord. A
// ping inside the gap still DRAWS -- it is a real ping and the rings are the
// truth of it -- it just does not get its own note.
const MIN_GAP = BLIP * 1000;

// AudioRamp is the slice of AudioParam the sound is scheduled onto, named here
// rather than taken as an AudioParam so that the SHAPE of the sound can be tested
// without an audio device. render/pings.ts exports ringAt for the same reason,
// and the reason is the same in both places: what can be wrong here is
// arithmetic over time, and arithmetic over time is what a headless test checks.
export interface AudioRamp {
	setValueAtTime(value: number, startTime: number): unknown;
	linearRampToValueAtTime(value: number, endTime: number): unknown;
	exponentialRampToValueAtTime(value: number, endTime: number): unknown;
}

// gainFor turns the setting into the level a note is struck at.
//
// IT IS SQUARED AND NOT LINEAR, because loudness is not. Halfway down a linear
// gain slider is 6 dB quieter, which reads as "slightly less"; halfway down this
// one is 12 dB, which is about half as loud to an ear -- so the middle of the
// track is the middle of the range somebody is actually choosing between. The
// alternative puts every useful setting in the bottom third of the travel.
//
// ZERO IS EXACTLY ZERO, which is the one value that has to be exact: it is the
// mute, and a curve that merely got very close to it would be a mute that could
// still be heard through headphones in a quiet room.
export function gainFor(volume: number): number {
	const percent = Number.isFinite(volume) ? Math.min(Math.max(volume, 0), FULL) : FULL;
	if (percent <= 0) {
		return 0;
	}

	const share = percent / FULL;

	return PEAK * share * share;
}

// voice schedules the whole sound: two pitches on one oscillator, and the level
// that makes them two notes rather than one.
//
// TWO NOTES, ONE OSCILLATOR, AND THE GAIN IS WHAT SEPARATES THEM. Every ramp
// below is scheduled against the value at the event before it, so the order is
// the shape:
//
//   0 -> peak     the first note is struck
//   held         through it, so it is actually heard
//   -> FLOOR     the breath between the two
//   -> peak      the second note is struck, a fifth higher
//   held
//   -> FLOOR     the only decay in the whole sound
//
// LINEAR EVERYWHERE EXCEPT THE LAST ONE. A linear move to and from nothing is
// what a struck note does at these lengths, and an exponential ramp cannot start
// from a value of zero anyway; the final tail is exponential because that is the
// one place there is time to hear the difference.
//
// THE PEAK IS A PARAMETER because the volume scales it, and the FLOOR is not:
// the dip between the notes and the tail after them are both "as close to nothing
// as a ramp may get", which is a property of the ramp rather than of how loud
// somebody wanted this.
export function voice(pitch: AudioRamp, level: AudioRamp, at: number, peak: number): void {
	pitch.setValueAtTime(LOW, at);
	pitch.setValueAtTime(HIGH, at + NOTE);

	level.setValueAtTime(0, at);
	level.linearRampToValueAtTime(peak, at + ATTACK);
	level.setValueAtTime(peak, at + NOTE - GAP);
	level.linearRampToValueAtTime(FLOOR, at + NOTE);
	level.linearRampToValueAtTime(peak, at + NOTE + ATTACK);
	level.setValueAtTime(peak, at + NOTE * 2);
	level.exponentialRampToValueAtTime(FLOOR, at + BLIP);
}

export interface PingSound {
	// volume is the account setting arriving, as a percentage. It is called with
	// what the room page rendered and again on every save of the settings dialog,
	// and it is the ONLY way this level is ever set -- nothing here reads or
	// writes storage.
	volume(percent: number): void;

	// play makes the noise, unless it is turned all the way down or the last one
	// is still sounding. The CALLER decides whether a ping deserves one at all --
	// see main.ts, which declines this viewer's own and any on a floor they are
	// not looking at.
	play(): void;

	stop(): void;
}

export function newPingSound(): PingSound {
	// FULL UNTIL TOLD OTHERWISE, for the reason on FULL: the setting arrives a
	// moment after this is built, and the wrong way to be wrong for that moment
	// is silence.
	let peak = gainFor(FULL);

	let context: AudioContext | null = null;

	// last is when the previous blip sounded, and it starts BEFORE TIME rather
	// than at zero. Zero is a real reading -- performance.now() counts from the
	// moment the page started loading -- so a nought here says "one sounded at
	// page load" and swallows every ping in the first MIN_GAP milliseconds of the
	// room's life. Nothing at a table can ping that early, which is exactly why
	// it would never have been noticed.
	let last = Number.NEGATIVE_INFINITY;

	// resumed answers a context that is ready, making one the first time it is
	// asked. A browser with no AudioContext at all, or one that refuses to build
	// one, answers null for ever after.
	function resumed(): AudioContext | null {
		if (context === null) {
			try {
				context = new AudioContext();
			} catch {
				return null;
			}
		}

		// A CONTEXT CAN BE SUSPENDED AT ANY POINT AND NOT ONLY AT THE START. A
		// tab in the background is the ordinary case, and coming back to the tab
		// is what resumes it -- so this is asked every time rather than once. The
		// promise is deliberately unawaited: a ping is worth a noise NOW or not
		// at all, and the frame after a resume settles is a frame too late to be
		// the sound of the thing that just happened.
		if (context.state === "suspended") {
			void context.resume().catch(() => {});

			// AND A VIEWER WHO HAS NOT TOUCHED THE PAGE CANNOT BE RESUMED AT ALL,
			// which is the case this arms for. Every browser refuses audio until
			// the page has been interacted with, and following a link into a room
			// is an interaction with the page somebody LEFT -- so a player who
			// joins and then sits watching the map is a player whose context
			// stays suspended however often it is asked. Their next press on
			// anything is what opens it, and from there every ping is heard.
			// Re-adding the same listener while one is pending does nothing, so
			// this does not stack up.
			document.addEventListener("pointerdown", wake, { once: true });
		}

		return context;
	}

	function wake(): void {
		void context?.resume().catch(() => {});
	}

	return {
		volume(percent) {
			peak = gainFor(percent);
		},

		play() {
			if (peak <= 0) {
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
				voice(osc.frequency, gain.gain, at, peak);

				osc.connect(gain).connect(ctx.destination);
				osc.start(at);
				osc.stop(at + BLIP);
			} catch {
				// An audio device that went away mid-session, which is a laptop
				// being unplugged from a dock. The table carries on.
			}
		},

		stop() {
			document.removeEventListener("pointerdown", wake);
			void context?.close().catch(() => {});
			context = null;
		},
	};
}
