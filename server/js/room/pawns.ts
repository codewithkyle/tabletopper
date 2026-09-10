// What a pointer on the table means: what is under it, what a drag does, and
// what placement puts down.
//
// THIS IS A STATE MACHINE AND THE STATES ARE THE FOUR THINGS A PRESS CAN
// BECOME. A press on a movable pawn is a click until the hand has moved four
// device pixels, at which point it is a drag; a press on a handle is a resize or
// a turn; a press on anything else -- empty table, or a pawn this viewer may not
// move -- is a click until the hand has travelled that same distance and a
// marquee after that.
//
// AND UNDER THE MOVE TOOL A PRESS IS NONE OF THEM. The pill has a mode that
// hands the pointer to the camera and takes it away from the table's contents,
// and the space bar is a hold of that mode; deps.panning is the whole of what
// this module knows about either. What is left alone by it is the selection --
// shoving the map around is not a reason to throw away the group somebody spent
// a minute building.
//
// UNDER THE MEASURE TOOL A PRESS IS A RULER, which is the one mode that takes
// the primary button and puts something of its own on the table. It is not a
// gesture: the point stays down when the button comes up and the line goes on
// following the pointer, because the question it answers -- how far is that --
// is asked with a hand off the mouse and an eye on the map. See measurement.
//
// NOTHING HERE ALLOCATES PER FRAME. The ghosts, the outlines and the ruler are
// written into arrays the caller owns and reuses, because they are read once
// per frame for as long as a hand is moving.
//
// THE DRAG IS PREVIEWED LOCALLY AND COMMITTED ONCE. Every client works out the
// same ghosts and the same path from the same two cells, so what crosses the
// wire while a hand is moving is one position a few times a second -- and the
// move itself is a single command at the end. See decision 7.

import type { Drawn } from "./render/pawn-pass.ts";
import type { Event, Grid, Pawn, PawnKind, Role, Size, State, Stroke } from "./protocol.ts";
import type { Modifiers, Tool } from "./render/input.ts";
import type { Outgoing } from "./socket.ts";
import type { Point, Rect } from "./render/camera.ts";
import { Selection, dragSet, marqueeSelect, mayMove } from "./selection.ts";
import type { Placed, Sized } from "./render/path.ts";
import {
	cellAt,
	cellCentre,
	boundsOf,
	cellsMoved,
	containsPoint,
	distanceLabel,
	feetBetween,
	pawnExtents,
	snapPawn,
	snapsToGrid,
	supercover,
} from "./render/path.ts";
import type { Draw } from "./draw.ts";
import type { Fog } from "./fog.ts";
import type { Handle } from "./handles.ts";
import { SPIN_STEP, handleAt, handlesFor, resized, turned } from "./handles.ts";
import { typing } from "./keys.ts";
import { compareStack } from "./render/scene.ts";

// DRAG_THRESHOLD is how far the hand moves before a press stops being a click,
// in DEVICE pixels. Four is under a millimetre and above the jitter of a hand
// resting on a mouse -- which is what it is for, because a click that
// accidentally moved a goblin one cell is a click nobody notices until the
// fight is over.
const DRAG_THRESHOLD = 4;

// DOUBLE_MS is how long the second click of a double click has to arrive in.
//
// IT IS COUNTED HERE RATHER THAN TAKEN FROM THE BROWSER'S OWN dblclick, and the
// reason is one line in input.ts: every primary pointerdown on the canvas is
// preventDefault-ed, because otherwise the middle button's scroll puck appears
// over the map and stays there. A prevented pointerdown suppresses the
// compatibility mouse events a dblclick is assembled from, and browsers do not
// agree about how much of that chain survives. Two presses on the same pawn is
// something this module already knows about, so it counts them itself.
//
// WHAT IS LOST BY COUNTING IS THE PLATFORM'S OWN DOUBLE CLICK SPEED, which is a
// real accessibility setting and is not readable from a page. 400 milliseconds
// is a little under the defaults, which sit around half a second, and is long
// enough that it is not a race.
const DOUBLE_MS = 400;

// DRAG_HZ is how often a drag reports itself when snapping is OFF. With
// snapping on the hovered cell is the clock and this never runs: the hot path
// quantises itself, which is the whole reason cell-change is the trigger.
const DRAG_INTERVAL = 1000 / 20;

// PREVIEW_TIMEOUT drops somebody else's ghosts when their drag stops arriving.
// A tab that was closed mid-drag, or a connection that went away, would
// otherwise leave a ghost on everybody's table until the room reloaded.
const PREVIEW_TIMEOUT = 3000;

// GHOST_ALPHA is how solid a previewed position is. Half, so the committed
// pawn underneath is still legible -- what a drag shows is a proposal, and a
// proposal that hid what it was replacing would be worse than no preview.
export const GHOST_ALPHA = 0.5;

// ACTOR_COLORS is how one dragging player is told from another.
//
// A FIXED PALETTE INDEXED BY A HASH OF THE PLAYER ID, so the colour is stable
// across a session and across a reload without anything being stored or sent.
// Six people at a table and eight colours means a collision is possible and
// harmless: two ghosts the same colour is two ghosts, which is what they are.
const ACTOR_COLORS: readonly (readonly [number, number, number])[] = [
	[0.36, 0.65, 0.98],
	[0.99, 0.6, 0.28],
	[0.42, 0.82, 0.5],
	[0.85, 0.44, 0.9],
	[0.98, 0.78, 0.3],
	[0.4, 0.85, 0.83],
	[0.95, 0.45, 0.5],
	[0.72, 0.72, 0.78],
];

// SELF_COLOR is this client's own ruler. It is deliberately not from the
// palette: your own drag is the one you are looking at, and it reads against
// both themes.
const SELF_COLOR: readonly [number, number, number] = [0.98, 0.98, 0.99];

// SELECT_COLOR is what the marquee, the outline round a selected pawn and the
// resize handles are all drawn in.
//
// IT IS BLUE BECAUSE WHITE HAS A MAP IT CANNOT BE SEEN ON. The near-white above
// reads against both themes, which is a statement about the page and not about
// the table: the canvas underneath is somebody's battle map, and a snowfield, a
// frozen lake or a sheet of fog is a near-white rectangle a near-white hairline
// disappears into. Hue survives what luminance does not -- there is no common
// map that is this blue over its whole surface -- and it stays light, so it is
// still legible over a dungeon floor.
//
// IT IS NOT ACTOR_COLORS[0], though the two are near neighbours. That palette
// says WHOSE drag a ghost is, and a selection saying "yours" in a colour the
// palette hands out would be a claim about a person rather than about the
// pointer. Selection is not a member of that set; it is the one thing on the
// table that belongs to whoever is looking.
export const SELECT_COLOR: readonly [number, number, number] = [0.4, 0.78, 1.0];

// MEASURE_POINT is the radius of the mark the measure tool leaves where it was
// clicked, in CSS pixels, and MEASURE_WIDTH is that ring's thickness in device
// pixels -- the same two units the resize handles are drawn in, and for the same
// reason: it is a control-sized thing rather than a table-sized one, so it stays
// the size of a fingertip at every zoom.
//
// A RING AND NOT A CROSS OR A PIN. The end of the ruler is under the pointer,
// which is already drawn by the operating system; what needs marking is the end
// that is not, and a small circle is the one shape on this canvas that is
// already spoken here -- a selection is a ring, a condition is a ring, a rotate
// handle is a ring.
const MEASURE_POINT = 4;
const MEASURE_WIDTH = 2;

// Outline is one ring or rectangle for the ring pass: a selection, a ghost, or
// the marquee.
export interface Outline {
	x: number;
	y: number;
	halfW: number;
	halfH: number;
	color: readonly [number, number, number];
	alpha: number;
	thickness: number;
	rect: boolean;

	// rotation is degrees clockwise about the centre, which an outline round a
	// turned token needs and the marquee never has.
	rotation: number;
}

// Segment is one straight line for the over-marks path pass, in map pixels: a
// side of the fog polygon being clicked out, or the rubber band to the pointer.
//
// IT IS NOT A Ruler, which carries the cells under it and a distance to print.
// A fog outline is a line and nothing else, and folding it into the ruler would
// have meant a Ruler with an empty label that highlights no squares.
export interface Segment {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	color: readonly [number, number, number];
	alpha: number;
	width: number;
}

// Ruler is one drag's path: the cells it crosses, the line across them, and how
// far that is.
export interface Ruler {
	cells: number[];
	x0: number;
	y0: number;
	x1: number;
	y1: number;
	label: string;
	color: readonly [number, number, number];
}

// Armed is what placement will put down on the next click, built by the spawn
// dialog out of the card that was picked and the one switch above it.
//
// IT IS ONLY EVER THE GM'S. Putting something on the table is refused for
// everybody else in PawnSpawn.Authorize, and the dialog that builds this is a
// GM-only fragment, so a player's canvas is never armed.
export interface Armed {
	kind: PawnKind;
	id: string;
	name: string;
	image: string;
	visible: boolean;

	// size is the creature size the ghost is drawn at, and it is ignored for an
	// object -- which is measured in pixels below.
	size: Size;

	// width and height are the PICTURE'S own pixels for an object, and zero
	// when the library row does not record them. Zero means one cell, which is
	// the same fallback the hub applies when it resolves the spawn, so the
	// ghost and the pawn that lands are the same size either way.
	//
	// THEY ARE NOT SENT. The spawn command carries no size at all: the hub
	// reads the assets row again and takes the answer from there. These exist
	// so the thing following the pointer is the thing about to be placed.
	width: number;
	height: number;

	// hp, maxHp and ac are the NPC form's three numbers, and they are the one
	// part of this spec that IS sent.
	//
	// AN NPC IS THE ONLY KIND WITH NOTHING TO READ. A monster's stat line is a
	// row in the manual and a character's is a row on the sheet, both of which
	// the hub reads for itself; a face out of the avatar library has neither,
	// so the numbers travel from the form that asked for them. They are zero
	// for every other kind and place() does not put them on the wire.
	hp: number;
	maxHp: number;
	ac: number;
}

// Ghostable is the part of a pawn a ghost is built from. It is a Pick rather
// than Pawn because the armed spec is not a pawn yet -- it has no id, no owner
// and no floor -- and casting one into a Pawn to draw it was a lie the compiler
// had to be talked out of.
type Ghostable = Pick<
	Drawn,
	"id" | "kind" | "name" | "image" | "x" | "y" | "z" | "size" | "width" | "height" | "rotation"
>;

export interface TableDeps {
	state: State;
	role: Role;
	user: string;
	viewed: () => string;
	send: (command: Outgoing) => void;
	invalidate: () => void;

	// scale is how many MAP pixels one CSS pixel covers, which is the camera's
	// zoom inverted. The resize handles are the only thing here that needs it:
	// they are a fixed size on screen and are therefore a moving size on the
	// table, and a hand grabbing one is aiming in screen pixels.
	scale: () => number;

	// details opens a pawn's window, which is what a DOUBLE click on one means.
	//
	// IT USED TO BE THE RIGHT BUTTON AND IS NOT ANY MORE. Playtesters reached
	// for the double click without being told to and found nothing under it,
	// which is the only kind of evidence about a gesture worth having. The
	// right button now puts up a menu, and Open details is the first thing on
	// it -- so the gesture people looked for works, and the one they were
	// taught still gets them there.
	//
	// IT IS A CALLBACK AND NOT A URL BUILT HERE. This module knows what is
	// under a pointer; it does not know which room it is in, what a fragment
	// path looks like, or how large a window opens. Those belong to the wiring
	// that already holds all three.
	details: (pawn: Pawn) => void;

	// menu is the right button asking about the pawn under it, at the point on
	// screen where the question was asked.
	//
	// THE POINT IS IN CSS PIXELS AND NOT MAP PIXELS, because what gets placed
	// there is a list of words: it is the size of its own text at every zoom,
	// and it is gone before the camera can move out from under it.
	//
	// WHAT IS ON IT IS NOT THIS MODULE'S BUSINESS, for the reason details is a
	// callback too. The items are a floor list and a removal, and both of those
	// are markup with a room id in it.
	menu: (pawn: Pawn, screen: Point) => void;

	// panning is whether the pointer belongs to the camera and to nothing
	// else, which is the Move tool in the pill and the space bar while it is
	// held. See tools.ts.
	//
	// IT IS ASKED AT THE MOMENT OF A PRESS AND NEVER AGAIN DURING ONE. A
	// gesture that has begun finishes under the tool it began in: letting go of
	// the space bar halfway through a marquee must not turn the box into a pan,
	// and pressing it halfway through a drag must not drop the goblin.
	//
	// WHAT IT TURNS OFF IS THE POINTER AND NOT THE PANEL. Move takes away
	// dragging, selecting, marqueeing and placing -- everything the primary
	// button does to the table's CONTENTS -- and leaves the selection somebody
	// built exactly where it was, because a mode for shoving the map around is
	// not a mode for throwing away their work.
	panning: () => boolean;

	// measuring is whether the ruler in the pill is the tool that was chosen.
	//
	// IT IS ASKED AT A PRESS AND ALSO ON THE WAY TO A FRAME, which is the one
	// difference from panning: a measurement stays on the table after the button
	// comes up, so what puts it away is the tool ceasing to be the chosen one.
	// Reading it per frame is how that happens without a subscription.
	//
	// THE SPACE BAR DOES NOT TURN IT OFF, deliberately. The hold borrows the
	// pointer for the camera and does not change which tool is chosen, so a GM
	// can shove the map along a corridor with the ruler still stretched across
	// it -- which is exactly what measuring something further away than the
	// screen is.
	measuring: () => boolean;

	// fog is the Fog tool, or null for a viewer who has none -- which is every
	// player, whose pill does not render the button.
	//
	// THE TABLE OWNS THE GESTURES AND THE FOG OWNS ITS OWN. Every pointer event
	// arrives here first because this is what input.ts was given, and the four
	// that mean something to a half-drawn shape are handed straight on. What
	// this module does NOT do is know what a fog shape is; see fog.ts.
	//
	// IT IS ALSO WHERE CONCEALMENT COMES FROM, for a player. The draw, the
	// hover, the hit test and the marquee all ask it whether a pawn is under
	// the cover, and all four ask the same function -- so a goblin a player
	// cannot see is a goblin they cannot label, click or sweep up either.
	fog: Fog | null;

	// draw is the pen, and it is here for the reason the fog is: every pointer
	// event arrives at this module first, and the ones that mean something to a
	// line in hand are handed straight on. What this module does NOT do is know
	// what a stroke is; see draw.ts.
	//
	// IT IS EVERYBODY'S, unlike the fog. Whether a player may actually draw is
	// the room's setting and the core's refusal, not a null here.
	draw: Draw | null;

	// remove is the Delete key asking for the selection to be taken off the
	// table.
	//
	// IT ASKS RATHER THAN SENDS, and that is the whole reason it is a callback
	// too. Removing pawns is confirmed, the confirmation is the app's confirm
	// modal, and that modal is driven by hx-confirm on the element making the
	// request -- so what this can do is press the button that already carries
	// it. window.confirm is banned and a second confirmation of our own would
	// be a fourth dialog.
	remove: () => void;
}

export interface Table {
	tool: Tool;
	selection: Selection;

	// focus is what the overlay is about, which is the HOVERED pawn and never a
	// selected one. See the overlay's own header: a label answers "what am I
	// pointing at", and that stops being a question the moment somebody has
	// picked the thing out.
	focus(): Pawn | null;

	ghosts(out: Drawn[]): Drawn[];
	outlines(out: Outline[]): Outline[];
	rulers(out: Ruler[]): Ruler[];

	// marks is the loose lines over the table: the fog polygon in hand, and
	// nothing else yet. They are drawn through the same pass the ruler's line
	// is, which is what puts them over everything including the cover.
	marks(out: Segment[]): Segment[];

	// inHand is the stroke this viewer is drawing right now, or null. It is
	// the LOCAL copy rather than the store's, which is what keeps the ink under
	// the pen instead of a round trip behind it; see draw.ts.
	inHand(): Stroke | null;

	// concealed is whether this viewer is shown nothing of a pawn, which is a
	// question about the fog and never about the pawn's own visible flag -- a
	// hidden pawn does not reach a player at all. It is false for every GM.
	concealed(pawn: Pawn): boolean;

	// handles is the resize and rotate controls, which exist for exactly one
	// selected object the viewer may move and are empty every other time.
	handles(out: Handle[]): Handle[];

	// bounds is the box the overlay sits above: one pawn's, or the selection's.
	bounds(): Rect | null;

	// preview consumes the events that start and end somebody else's ghosts.
	preview(event: Event): void;

	// floorChanged is the renderer settling on another floor. A selection is of
	// things the viewer can see, and one that survived a floor change would be
	// pawns the outlines no longer draw and the Delete key still removes.
	floorChanged(): void;

	arm(armed: Armed | null): void;
	isArmed(): boolean;

	// onChange fires when the overlay's CONTENTS need rewriting, which is a
	// different question from whether the canvas needs a frame: the overlay's
	// text is set on a change and its position on every frame.
	onChange(fn: () => void): void;

	stop(): void;
}

// A press that has not yet decided what it is.
interface Pressing {
	kind: "press";
	anchor: string;
	screen: Point;
	grab: Point;
	mods: Modifiers;
}

interface Dragging {
	kind: "drag";
	anchor: string;
	ids: string[];
	origins: Map<string, Point>;
	grab: Point;
	ghost: Point;
	sent: [number, number] | null;
	sentAt: number;
}

// Shaping is a hand on a resize or rotate handle. It carries the proposal
// rather than writing it to the pawn: the preview is a ghost, and the whole
// gesture leaves as one PawnUpdate on release.
//
// THE HANDLE IS COPIED AND NOT REFERENCED. handlesFor writes into an array it
// reuses every frame, so holding the object it handed back would be holding a
// slot that has since become a different handle.
interface Shaping {
	kind: "shape";
	id: string;
	handle: Handle;
	width: number;
	height: number;
	rotation: number;

	// moved is whether the hand actually asked for anything. A click that
	// landed on a handle and went nowhere sends nothing.
	moved: boolean;
}

// Marqueeing is a press on empty table, or on a pawn this viewer may not move:
// a click until the hand has travelled, and a rubber band around everything it
// crosses after that.
//
// IT IS ONE GESTURE AND NOT TWO BECAUSE THE HAND DOES NOT DECLARE WHICH IT
// MEANT. A press on empty floor is a click that clears the selection and a
// group selection that has not started moving yet, and which of them it was is
// only known when the button comes back up -- the same shape as a press on a
// goblin, which is a selection until it has moved four pixels and a drag after
// that.
//
// THE SELECT TOOL TAKES THIS PRESS AWAY FROM THE CAMERA, which is the trade the
// tool makes: the primary button draws boxes, and panning is the space bar, the
// middle button, or the Move tool. Under Move there is no gesture here at all.
interface Marqueeing {
	kind: "marquee";
	from: Point;
	to: Point;

	// screen is where the press was in CSS pixels, which is what the threshold
	// is measured in -- a hand's jitter is a screen-space quantity and does not
	// get larger as the map is zoomed out.
	screen: Point;
	moved: boolean;

	// anchor is what the press landed on, whether or not this viewer may move
	// it, and null for empty table.
	//
	// IT EXISTS FOR THE DOUBLE CLICK. A player asking to read a monster gets no
	// Pressing gesture -- a monster is not theirs to drag -- and without this
	// the pair could not be counted for the one viewer most likely to be making
	// it.
	anchor: string | null;
}

type Gesture = Pressing | Dragging | Shaping | Marqueeing | null;

// Measured is the measure tool's ruler: where it was put down, and where the
// pointer has got to since.
//
// IT IS NOT A GESTURE AND THAT IS THE WHOLE POINT OF IT. Every one of the four
// above lives between a press and a release; this one is put down by a press and
// outlives it, so a GM can let go of the mouse, lean over the table and read the
// number. What ends it is another press, or Escape, or the right button, or
// choosing another tool.
//
// SO THE TOOL IS TWO CLICKS AND THE SECOND ONE IS THE FULL STOP. Down, read,
// up: the same shape as the two clicks that measure a wall in every CAD program
// there has ever been, and the gesture is over where the eye already is rather
// than at a key. The three ways out are still there and still do the same
// thing; this is the one that is already under the hand.
//
// BOTH POINTS ARE RAW MAP PIXELS AND NEITHER IS SNAPPED. A ruler is not a move:
// a creature walks in squares, and a fireball's radius, a bow's range and the
// gap between two bits of scenery do not. See feetBetween.
interface Measured {
	from: Point;
	to: Point;
}

// Preview is somebody else's drag, as it arrived.
interface Preview {
	positions: { id: string; x: number; y: number }[];
	color: readonly [number, number, number];
	at: number;
}

export function createTable(deps: TableDeps): Table {
	const { state, role, user } = deps;

	const selection = new Selection();
	const previews = new Map<string, Preview>();

	let gesture: Gesture = null;
	let hovered: string | null = null;
	let armed: Armed | null = null;
	let pointer: Point | null = null;
	let measured: Measured | null = null;

	// fogging is whether the pointer that is down was taken by the FOG rather
	// than by anything in this file. It is not "is the Fog tool chosen", which
	// is deps.fog's own question: a gesture that began under the fog finishes
	// under it whatever the pill does halfway through, which is the rule every
	// other mode here follows.
	let fogging = false;

	// inking is the same flag for the pen: whether the pointer that is down was
	// taken by draw.ts. A gesture that began under the pen finishes under it,
	// whatever the pill does halfway through.
	let inking = false;

	// lastClick is half of a double click: which pawn, and when. See countClick
	// for what breaks a pair.
	let lastClick: { id: string; at: number } | null = null;
	let changed: (() => void) | null = null;

	// Scratch, so nothing in the frame path allocates.
	const snapped: Point = { x: 0, y: 0 };
	const handleList: Handle[] = [];
	const shape: Placed = {
		kind: "object", size: "medium", width: 0, height: 0, rotation: 0, x: 0, y: 0,
	};
	const proposal: Ghostable = {
		id: "", kind: "object", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium", width: 0, height: 0, rotation: 0,
	};

	function grid(): Grid {
		return state.table.grid;
	}

	function pawn(id: string): Pawn | null {
		return state.pawns.find((p) => p.id === id) ?? null;
	}

	function announce(): void {
		changed?.();
		deps.invalidate();
	}

	// measurement is the ruler the measure tool put down, and null the moment
	// that tool is not the chosen one any more.
	//
	// IT FORGETS RATHER THAN HIDES, which is what makes leaving the tool and
	// coming back a fresh start instead of a resurrected line somebody drew
	// before the fight moved. Doing it here rather than on a change event is
	// what keeps tools.ts from having to notify anybody: every reader of a
	// measurement goes through this function, so there is no order in which a
	// stale one can be drawn.
	function measurement(): Measured | null {
		if (measured && !deps.measuring()) {
			measured = null;
		}

		return measured;
	}

	// aim moves the far end of the ruler to the pointer. It is called from the
	// drag AND from the hover, because a measurement follows the cursor whether
	// or not a button is down -- the click that started it is the only press it
	// wants.
	function aim(map: Point): void {
		const line = measurement();
		if (!line) {
			return;
		}

		line.to.x = map.x;
		line.to.y = map.y;
		deps.invalidate();
	}

	// THE FLOOR IS THE FILTER AND IT IS APPLIED IN ONE PLACE. Hit testing, the
	// marquee, the riders lookup and the hover all go through here, so a pawn
	// on another floor is not reachable by any of them.
	function onFloor(p: Pawn): boolean {
		return p.layerId === deps.viewed();
	}

	// shaped is a pawn as the viewer's own hand is currently proposing it: the
	// committed pawn, with a live resize or rotation written over the top.
	//
	// IT IS ONE SCRATCH OBJECT, reused, because the outline, the handles and the
	// ghost all ask for it on every frame of a gesture.
	function shaped(p: Pawn): Placed {
		shape.kind = p.kind;
		shape.size = p.size;
		shape.x = p.x;
		shape.y = p.y;
		shape.width = p.width;
		shape.height = p.height;
		shape.rotation = p.rotation;

		if (gesture?.kind === "shape" && gesture.id === p.id) {
			shape.width = gesture.width;
			shape.height = gesture.height;
			shape.rotation = gesture.rotation;
		}

		return shape;
	}

	// selectedToken is the one token the handles belong to, and activeHandles is
	// the controls that are up right now.
	//
	// ONE SELECTED OBJECT AND NOT A HOVERED ONE. Handles are a commitment: they
	// sit under the pointer and take a press that would otherwise have moved the
	// token, so they appear when somebody has said which token they mean.
	//
	// A MULTIPLE SELECTION HAS NONE either, because scaling six things about six
	// different centres is not one gesture, and scaling them about a shared one
	// would move five of them.
	function selectedToken(): Pawn | null {
		const one = selection.only();
		const p = one ? pawn(one) : null;

		if (!p || p.kind !== "object" || !onFloor(p) || !mayMove(p, role, user)) {
			return null;
		}

		return p;
	}

	// described is the pawn the overlay labels: the hovered one, and never a
	// token.
	//
	// A TOKEN HAS NOTHING TO SAY. A rug, a road, a wagon: no hit points, no
	// armour class, and a name the picture already tells you. What a label over
	// one WOULD do is sit on top of the resize and rotate handles that appear
	// the moment it is selected.
	function described(): Pawn | null {
		const p = hovered ? pawn(hovered) : null;

		return p && p.kind !== "object" ? p : null;
	}

	function activeHandles(out: Handle[]): Handle[] {
		const p = selectedToken();
		if (!p) {
			out.length = 0;

			return out;
		}

		return handlesFor(shaped(p), grid().cellSize, deps.scale(), out);
	}

	function snapFor(p: Sized, x: number, y: number): Point {
		const [sx, sy] = snapPawn(grid(), p, Math.round(x), Math.round(y));
		snapped.x = sx;
		snapped.y = sy;

		return snapped;
	}

	function beginDrag(from: Pressing): void {
		const anchor = pawn(from.anchor);
		if (!anchor) {
			gesture = null;

			return;
		}

		const ids = dragSet(state.pawns, anchor, selection, {
			// ALT TAKES THE WAGON OUT FROM UNDER ITS RIDERS, which is the one
			// thing the automatic rule cannot express on its own.
			withRiders: !from.mods.alt,
			role,
			user,
			cellSize: grid().cellSize,
		});

		const origins = new Map<string, Point>();
		for (const id of ids) {
			const p = pawn(id);
			if (p) {
				origins.set(id, { x: p.x, y: p.y });
			}
		}

		gesture = {
			kind: "drag",
			anchor: anchor.id,
			ids,
			origins,
			grab: from.grab,
			ghost: { x: anchor.x, y: anchor.y },
			sent: null,
			sentAt: 0,
		};
	}

	// moveDrag places the anchor's ghost and reports the drag when the cell it
	// is over has changed.
	function moveDrag(active: Dragging, map: Point): void {
		const anchor = pawn(active.anchor);
		if (!anchor) {
			return;
		}

		const point = snapFor(anchor, map.x + active.grab.x, map.y + active.grab.y);
		active.ghost.x = point.x;
		active.ghost.y = point.y;

		const cell = cellAt(grid(), point.x, point.y);
		const now = performance.now();

		// WITH SNAPPING ON THE CELL IS THE CLOCK, which is what makes the hot
		// path quantise itself: a fast mouse and a slow one cross the same
		// boundaries and send the same number of frames. With it off there is no
		// boundary to cross, so a timer stands in.
		const moved = !active.sent || cell[0] !== active.sent[0] || cell[1] !== active.sent[1];
		const due = snapsToGrid(anchor, grid()) ? moved : now - active.sentAt >= DRAG_INTERVAL;
		if (!due) {
			return;
		}

		active.sent = cell;
		active.sentAt = now;

		deps.send({
			type: "pawn.drag",
			anchor: active.anchor,
			x: point.x,
			y: point.y,
			others: followers(active),
		});
	}

	// commit ends a drag, at the ghost or back where it started.
	//
	// A CANCELLED DRAG SENDS THE COMMITTED POSITION RATHER THAN NOTHING. The
	// server answers with pawn.moved carrying unchanged positions, and that
	// event is what tells everybody else to drop the ghosts they are drawing --
	// so Escape needs no event of its own. See decision 8.
	function commit(active: Dragging, cancelled: boolean): void {
		const anchor = pawn(active.anchor);
		const origin = active.origins.get(active.anchor);
		gesture = null;

		if (!anchor || !origin) {
			return;
		}

		const x = cancelled ? origin.x : active.ghost.x;
		const y = cancelled ? origin.y : active.ghost.y;

		deps.send({
			type: "pawn.move",
			anchor: active.anchor,
			x,
			y,
			others: followers(active),
		});

		announce();
	}

	// followers is the rest of a drag set, minus anything that has left the
	// table since the drag began. The server answers a move naming a pawn that
	// is gone with not_found for the whole move, so six goblins dragged across
	// the room would snap back on release because somebody else removed one.
	function followers(active: Dragging): string[] {
		return active.ids.filter((id) => id !== active.anchor && pawn(id) !== null);
	}

	// moveShape is a hand on a handle: it works out what the pointer is asking
	// for and keeps it as a proposal. Nothing is sent while it runs.
	//
	// NOBODY ELSE SEES IT HAPPEN, which is the one way this is unlike a move
	// drag. pawn.dragging carries positions and only positions, and a preview of
	// a resize would be a second hot-path event for a gesture that lasts a
	// second and happens between fights. What the others get is the result.
	function moveShape(active: Shaping, map: Point, mods: Modifiers): void {
		const p = pawn(active.id);
		if (!p) {
			return;
		}

		if (active.handle.turns) {
			active.rotation = turned(p, map.x, map.y, mods.shift ? SPIN_STEP : 0);
		} else {
			const [width, height] = resized(p, active.handle, map.x, map.y, grid().cellSize, mods.shift);
			active.width = width;
			active.height = height;
		}

		active.moved = true;
		announce();
	}

	// commitShape ends a handle drag. Unlike a move, a cancelled one sends
	// NOTHING: the proposal never left this client, so there is nothing anybody
	// else has to be told to stop drawing.
	function commitShape(active: Shaping, cancelled: boolean): void {
		const p = pawn(active.id);
		gesture = null;

		if (!p || cancelled || !active.moved) {
			announce();

			return;
		}

		if (active.handle.turns) {
			if (active.rotation !== p.rotation) {
				deps.send({ type: "pawn.update", id: p.id, rotation: active.rotation });
			}
		} else if (active.width !== p.width || active.height !== p.height) {
			deps.send({ type: "pawn.update", id: p.id, width: active.width, height: active.height });
		}

		announce();
	}

	function place(map: Point): void {
		if (!armed) {
			return;
		}

		const point = snapFor(armedShape(armed, grid().cellSize), map.x, map.y);

		// NO SIZE FOR AN OBJECT AND NO FOOTPRINT FOR ANYTHING. What an object
		// is, is the picture named by assetId, and how big it is, is a column
		// beside that picture -- so there is nothing here for a browser to say
		// about it and nothing for the server to have to distrust.
		//
		// AND A STAT LINE FOR AN NPC AND NOTHING ELSE, for the mirror of that
		// reason: a face has no row to read hit points off, and every other
		// kind does. The server holds these to the same limits as a number
		// typed into a pawn's own panel; what stops a silly one leaving here is
		// the dialog, which will not arm on a field the browser refuses.
		const npc = armed.kind === "npc";

		deps.send({
			type: "pawn.spawn",
			kind: armed.kind,
			layer: deps.viewed(),
			x: point.x,
			y: point.y,
			visible: armed.visible,
			size: armed.kind === "object" ? undefined : armed.size,
			monsterId: armed.kind === "monster" ? armed.id : undefined,
			characterId: armed.kind === "player" ? armed.id : undefined,
			assetId: npc || armed.kind === "object" ? armed.id || undefined : undefined,
			name: npc || armed.kind === "object" ? armed.name : undefined,
			hp: npc ? armed.hp : undefined,
			maxHp: npc ? armed.maxHp : undefined,
			ac: npc ? armed.ac : undefined,
		});
	}

	// abandon is the one way out, and Escape and the right button are the two
	// ways to ask for it. It answers whether there was anything to abandon,
	// which is what tells the right button whether it has already been spent.
	//
	// THE ORDER MATTERS. A GM placing an encounter presses it to stop placing,
	// and a GM mid-drag presses it to put the pawn back. Placement wins, because
	// it is the mode you are IN rather than the gesture you are making.
	// concealed is the fog's answer about one pawn, and it is a named function
	// rather than four inline arrows so that the draw, the hover, the hit test
	// and the marquee cannot drift apart -- which is the failure this feature
	// has: a creature nobody can see that a marquee still picks up.
	function concealed(pawn: Pawn): boolean {
		return deps.fog?.concealed(pawn) ?? false;
	}

	function abandon(): boolean {
		if (armed) {
			arm(null);

			return true;
		}

		// AND THE FOG IS ASKED BEFORE THE RULER, for the reason placement is
		// asked before both: a half-drawn polygon is the most recent thing the
		// hand started, and Escape means the most recent thing.
		if (deps.fog?.abandon()) {
			fogging = false;

			return true;
		}

		// AND THE PEN IS ASKED BESIDE THE FOG, for the same reason. Escape mid
		// stroke is the most recent thing the hand started -- and abandoning a
		// line is not "send nothing", because the first point went out on the
		// press and is already on every other screen. See draw.ts.
		if (deps.draw?.abandon()) {
			inking = false;

			return true;
		}

		// AND A RULER IS PUT AWAY THE SAME WAY EVERYTHING ELSE IS. It is the
		// only one of these that outlives the button that made it, so it is
		// also the only one somebody has to ask to be rid of -- and the two
		// gestures they already know for "stop that" are the two that arrive
		// here.
		if (measurement()) {
			measured = null;
			announce();

			return true;
		}

		switch (gesture?.kind) {
			case "drag":
				commit(gesture, true);

				return true;

			case "shape":
				commitShape(gesture, true);

				return true;

			case "marquee":
				gesture = null;
				announce();

				return true;

			default:
				return false;
		}
	}

	// countClick records a click on a pawn and answers whether it completed a
	// pair. Null is a gesture that was not a click on anything, and it breaks
	// whatever pair was half made: click, quick drag, click is three things
	// that happened rather than one gesture.
	//
	// A THIRD CLICK IS NOT A SECOND DOUBLE. Forgetting the pair on the way out
	// is what keeps a finger resting on the button from opening the same window
	// again and again.
	function countClick(id: string | null): boolean {
		const now = performance.now();

		if (id === null) {
			lastClick = null;

			return false;
		}

		if (lastClick !== null && lastClick.id === id && now - lastClick.at <= DOUBLE_MS) {
			lastClick = null;

			return true;
		}

		lastClick = { id, at: now };

		return false;
	}

	// clickedPawn is what a finished gesture was a click ON, and null for every
	// gesture that was not one. Two of the four can end in a click and they end
	// in it differently: a press is a pawn this viewer may move, and a marquee
	// that never opened is either empty table or a pawn they may not.
	function clickedPawn(active: Gesture): Pawn | null {
		if (active?.kind === "press") {
			return pawn(active.anchor);
		}

		if (active?.kind === "marquee" && !active.moved && active.anchor !== null) {
			return pawn(active.anchor);
		}

		return null;
	}

	// THE KEYS ARE HEARD ON THE DOCUMENT AND THE TABLE IS NOT THE ONLY THING ON
	// IT. A GM typing a goblin's new name into a pawn window is pressing Delete
	// to rub out a letter, not to rub out the goblin -- so a key that arrives
	// from a field is not a key pressed on the table. Escape is asked the same
	// question for the same reason: it is Cancel in a form long before it is
	// "put that pawn back".
	function onKeyDown(e: KeyboardEvent): void {
		if (typing(e.target)) {
			return;
		}

		// ENTER, BACKSPACE AND CTRL-Z BELONG TO THE FOG WHILE THE FOG TOOL IS
		// CHOSEN, and to nothing at all otherwise -- fog.key answers false for
		// every other tool, so none of the three is taken from the page.
		if (deps.fog?.key(e)) {
			return;
		}
		if (deps.draw?.key(e)) {
			return;
		}

		if (e.key === "Escape") {
			abandon();

			return;
		}

		// DELETE IS THE SELECTION'S AND NOT THE HOVER'S. Everything else here
		// acts on what the pointer is over; this one does not, because a key that
		// removed whatever the mouse happened to be resting on is a key nobody
		// would press twice. There is no test of the role either: deps.remove
		// presses a control that exists for the GM alone, so a player pressing
		// Delete finds nothing to press.
		if (e.key === "Delete" && selection.size > 0) {
			deps.remove();
		}
	}

	function arm(next: Armed | null): void {
		armed = next;
		announce();
	}

	document.addEventListener("keydown", onKeyDown);

	const tool: Tool = {
		press(map, screen, mods) {
			pointer = { x: map.x, y: map.y };

			// THE CAMERA'S MODE IS ANSWERED FIRST AND WITHOUT RECORDING
			// ANYTHING, which is what makes it the mode it says it is: no
			// placement, no drag, no marquee, and no gesture left behind for the
			// release to act on. Returning false is how input.ts is told to pan
			// with this pointer.
			if (deps.panning()) {
				gesture = null;

				return false;
			}

			if (armed) {
				place(map);

				// THE MODE SURVIVES A SPAWN, which is the whole reason arming
				// is worth a round trip: an encounter is eight goblins and
				// eight clicks, not eight visits to a dialog.
				return true;
			}

			// AND THE FOG TAKES IT NEXT, above the ruler for the reason the
			// ruler is above the hit test: everything below this line touches
			// the table's CONTENTS, and a GM cutting fog is not aiming at a
			// goblin. It is below placement because arming is the mode you are
			// most recently in.
			//
			// THE GESTURE IS REMEMBERED HERE AND NOWHERE ELSE. input.ts sends
			// the drag and the release to whoever took the press, and this flag
			// is what routes them past four branches that know nothing of fog.
			if (deps.fog?.press(map, mods)) {
				fogging = true;
				gesture = null;

				return true;
			}

			// AND THE PEN NEXT, in the pill's own order and above the ruler for
			// the ruler's reason: a hand drawing on the map is not aiming at a
			// goblin either.
			if (deps.draw?.press(map, mods)) {
				inking = true;
				gesture = null;

				return true;
			}

			// THE RULER TAKES THE PRESS AND LEAVES NO GESTURE BEHIND, so the
			// release that follows finds nothing to finish: the point is down
			// and the line follows the pointer from here whether the button is
			// held or not. The SECOND press is the end of it, which makes the
			// tool the two clicks a wall is measured with everywhere else -- and
			// leaves Escape, the right button and choosing another tool doing
			// exactly what they already did, which is abandon.
			//
			// IT IS BELOW PLACEMENT because arming is a mode you are IN and the
			// tool is a mode you are in as well: a GM who has just picked a
			// monster out of the spawn dialog means to put it down, whatever
			// the pill says. It is above the handles and the hit test because
			// everything below this line touches the table's contents, which is
			// the half of the table a ruler is not allowed to disturb.
			if (deps.measuring()) {
				measured = measurement()
					? null
					: { from: { x: map.x, y: map.y }, to: { x: map.x, y: map.y } };

				gesture = null;

				return true;
			}

			// A HANDLE IS TESTED BEFORE THE TABLE IS, because a handle sits ON
			// the edge of the token it belongs to: whichever of the two the
			// press is nearer, somebody aiming at a five-pixel box meant the
			// box. Only the selected object has any, so this costs nine
			// distance tests and only while something is selected.
			const target = selectedToken();
			const grabbed = target ? handleAt(activeHandles(handleList), map.x, map.y, deps.scale()) : null;

			if (target && grabbed) {
				gesture = {
					kind: "shape",
					id: target.id,
					handle: { ...grabbed },
					width: target.width,
					height: target.height,
					rotation: target.rotation,
					moved: false,
				};

				return true;
			}

			const hit = hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed);

			if (hit && mayMove(hit, role, user)) {
				gesture = {
					kind: "press",
					anchor: hit.id,
					screen: { x: screen.x, y: screen.y },
					grab: { x: hit.x - map.x, y: hit.y - map.y },
					mods,
				};

				return true;
			}

			// Empty table, or something this viewer may not move. It is a
			// click that clears the selection until the hand travels, and a
			// marquee after that -- and the camera is refused either way,
			// because a select tool whose drag panned would have no gesture
			// left to draw a box with.
			gesture = {
				kind: "marquee",
				from: { x: map.x, y: map.y },
				to: { x: map.x, y: map.y },
				screen: { x: screen.x, y: screen.y },
				moved: false,
				anchor: hit?.id ?? null,
			};

			return true;
		},

		drag(map, screen, mods) {
			pointer = { x: map.x, y: map.y };

			aim(map);

			if (fogging) {
				deps.fog?.drag(map, mods);

				return;
			}

			if (inking) {
				deps.draw?.drag(map, mods);

				return;
			}

			// A PRESS BECOMES A DRAG HERE AND NOWHERE ELSE, which is what keeps
			// a click from moving anything: under four device pixels this
			// returns without touching the table, and the release that follows
			// is a selection.
			if (gesture?.kind === "press") {
				if (!past(gesture.screen, screen)) {
					return;
				}

				beginDrag(gesture);
			}

			// Re-read, because beginDrag replaced it.
			const active = gesture;
			if (!active) {
				return;
			}

			switch (active.kind) {
				case "drag":
					moveDrag(active, map);

					return;

				case "shape":
					moveShape(active, map, mods);

					return;

				case "marquee":
					// A PRESS BECOMES A BOX ON THE SAME THRESHOLD A PRESS ON A
					// GOBLIN BECOMES A DRAG. Under it there is nothing to draw
					// and nothing to select: the release is a click, and a hand
					// that shook while clicking empty table has not asked for a
					// selection of whatever it shook over.
					if (!active.moved && !past(active.screen, screen)) {
						return;
					}

					active.moved = true;
					active.to.x = map.x;
					active.to.y = map.y;
					announce();

					return;
			}
		},

		release(map, screen, mods) {
			pointer = { x: map.x, y: map.y };

			if (fogging) {
				fogging = false;
				deps.fog?.release(map, mods);

				return;
			}

			if (inking) {
				inking = false;
				deps.draw?.release(map, mods);

				return;
			}

			const active = gesture;
			gesture = null;

			if (!active) {
				return;
			}

			// ASKED ONCE, FOR EVERY KIND OF GESTURE, AND BEFORE ANY OF THEM ARE
			// HANDLED. A drag, a resize and a marquee all answer null here,
			// which is what makes the pair break by itself rather than by a
			// line remembering to break it in each of the branches below.
			const clicked = clickedPawn(active);
			const twice = countClick(clicked?.id ?? null);

			switch (active.kind) {
				case "drag":
					commit(active, false);

					return;

				case "shape":
					commitShape(active, false);

					return;

				case "press": {
					// It stayed a click. Shift toggles one pawn in or out;
					// anything else selects it alone.
					if (!clicked) {
						return;
					}

					if (mods.shift) {
						selection.toggle(clicked.id);
					} else {
						selection.set([clicked.id]);
					}
					announce();

					// AND THE SECOND OF A PAIR OPENS THE PAWN. Shift is left
					// out of it: two shift clicks on one pawn put it into a
					// selection and take it straight back out, which is
					// something somebody does on purpose, and a window landing
					// on the table halfway through picking a group is not.
					if (twice && !mods.shift) {
						deps.details(clicked);
					}

					return;
				}

				case "marquee": {
					if (!active.moved) {
						// It stayed a click. On empty table that is how a
						// selection is put down -- and with Shift it is not,
						// because Shift is building one and a miss is a miss.
						if (!mods.shift && selection.clear()) {
							announce();
						}

						// A PAWN THIS VIEWER MAY NOT MOVE IS STILL A PAWN THEY
						// MAY READ, which is the whole reason this gesture
						// carries an anchor. A player double-clicking a monster
						// gets its window; the selection they never had is
						// still cleared above.
						if (twice && !mods.shift && clicked) {
							deps.details(clicked);
						}

						return;
					}

					const rect = marqueeRect(active);
					const found = marqueeSelect(state.pawns, deps.viewed(), rect, role, user, concealed);

					if (mods.shift) {
						selection.add(found);
					} else {
						selection.set(found);
					}
					announce();

					return;
				}
			}
		},

		cancel() {
			// A POINTER THE BROWSER TOOK AWAY PUTS A HALF-DRAWN RECTANGLE BACK.
			// A polygon is not in the pointer's hands -- it is a list of clicks
			// and it survives -- and fog.abandon is what knows the difference.
			if (fogging) {
				fogging = false;
				deps.fog?.abandon();

				return;
			}

			// A POINTER THE BROWSER TOOK AWAY MID-STROKE RUBS THE LINE OUT, and
			// that is draw.abandon rather than an end: a half-line nobody meant
			// to draw is not a line worth leaving on everybody's table.
			if (inking) {
				inking = false;
				deps.draw?.abandon();

				return;
			}

			const active = gesture;
			gesture = null;

			// A pointer the browser took away is not a click, so it is not half
			// of a double one either.
			countClick(null);

			if (active?.kind === "drag") {
				commit(active, true);

				return;
			}
			if (active?.kind === "shape") {
				commitShape(active, true);

				return;
			}

			announce();
		},

		// THE RIGHT BUTTON IS A WAY OUT BEFORE IT IS A WAY IN. A GM halfway
		// through placing an encounter who right-clicks a goblin meant to stop
		// placing; putting a menu over the table they were working on would be
		// the opposite of what the press asked for. So abandoning spends the
		// click, and only a click with nothing to abandon asks about what is
		// under it.
		//
		// WHAT IT ASKS FOR IS A MENU AND NOT A WINDOW. Opening the editor was
		// what this used to do, and it is now what the menu's first item does:
		// the other two are moving the pawn to another floor and taking it off
		// the table, neither of which had a home that was not a keyboard key or
		// a control that only appears for a multiple selection.
		//
		// IT DOES NOT SELECT. A right click is a question about one pawn, and
		// answering it by throwing away whatever the GM had selected would make
		// "let me look at that" a destructive gesture.
		secondary(map, screen) {
			// THE FOG IS ASKED FIRST AND IT CAN ANSWER TWO WAYS, which is the
			// one place on this table where the right button means two things.
			// A polygon in hand CLOSES on it -- the gesture every mapping tool
			// has, and the one this table was asked for -- and a rectangle in
			// hand is abandoned by it, which is what the right button means
			// everywhere else here. With nothing in hand it declines and the
			// click goes on to mean what it always meant.
			if (deps.fog?.secondary()) {
				fogging = false;

				return;
			}

			if (deps.draw?.secondary()) {
				inking = false;

				return;
			}

			if (abandon()) {
				return;
			}

			const hit = hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed);
			if (hit) {
				// THE POINT IS COPIED. input.ts hands the same scratch object
				// to every call, and a menu that read it a frame later would
				// read wherever the pointer had got to.
				deps.menu(hit, { x: screen.x, y: screen.y });
			}
		},

		hover(map) {
			pointer = map ? { x: map.x, y: map.y } : null;
			deps.fog?.hover(map);
			deps.draw?.hover(map);

			if (map) {
				aim(map);
			}

			// AND THE HIT TEST STILL RUNS UNDER THE RULER. Measuring does not
			// take away reading the table: the label over the pawn a GM is
			// stretching a line towards is half of what makes the number mean
			// anything.
			const found = map ? hitTest(state.pawns, deps.viewed(), grid(), map.x, map.y, concealed) : null;
			const next = found?.id ?? null;

			if (next !== hovered) {
				hovered = next;
				announce();
			}
		},

		// A HALF-DRAWN FOG POLYGON IS NOT IN HERE, deliberately. What active()
		// buys is a frame loop that keeps running with nothing moving -- a
		// paused drag, a placement waiting for a click -- and a polygon needs
		// none of it: the rubber band follows the POINTER, every pointermove
		// asks for a frame of its own, and a GM who stops to think about the
		// next corner would otherwise leave the tab rendering at sixty frames a
		// second until they made up their mind.
		active() {
			return gesture !== null || armed !== null || previews.size > 0;
		},
	};

	function marqueeRect(active: Marqueeing): Rect {
		return { x1: active.from.x, y1: active.from.y, x2: active.to.x, y2: active.to.y };
	}

	function ghostOf(p: Ghostable, x: number, y: number, out: Drawn[], count: number): number {
		const slot = out[count] ?? (out[count] = blankDrawn());

		slot.id = p.id;
		slot.kind = p.kind;
		slot.name = p.name;
		slot.image = p.image;
		slot.x = x;
		slot.y = y;
		slot.z = p.z;
		slot.size = p.size;
		slot.width = p.width;
		slot.height = p.height;
		slot.rotation = p.rotation;
		slot.hidden = false;
		slot.health = null;

		return count + 1;
	}

	return {
		tool,
		selection,

		focus: described,

		ghosts(out) {
			// A DRAG THAT STOPPED ARRIVING IS DROPPED HERE, on the frame that
			// reads it. A tab closed mid-drag, or a connection that went away,
			// would otherwise leave a ghost on everybody's table for the rest of
			// the session -- and, because a live preview is one of the things
			// that keeps the frame loop awake, it would leave every client
			// rendering for ever to draw it.
			expirePreviews(previews, performance.now());

			let count = 0;

			// This client's own drag.
			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					const dx = gesture.ghost.x - origin.x;
					const dy = gesture.ghost.y - origin.y;

					for (const id of gesture.ids) {
						const p = pawn(id);
						const from = gesture.origins.get(id);
						if (p && from) {
							// ONE DELTA FOR EVERYBODY AND ONLY THE ANCHOR
							// SNAPS. A wagon with three people on it must arrive
							// with them sitting in the same three spots, and
							// snapping each one on its own would shuffle them
							// into the wagon's cells. The server applies the
							// same rule to the committed move.
							count = ghostOf(p, from.x + dx, from.y + dy, out, count);
						}
					}
				}
			}

			// Everybody else's.
			for (const preview of previews.values()) {
				for (const at of preview.positions) {
					const p = pawn(at.id);
					if (p && onFloor(p)) {
						count = ghostOf(p, at.x, at.y, out, count);
					}
				}
			}

			// A RESIZE OR A ROTATION IN PROGRESS, drawn as the same kind of
			// proposal a move is: the committed token stays where it is and the
			// ghost shows what letting go would do.
			if (gesture?.kind === "shape" && gesture.moved) {
				const p = pawn(gesture.id);
				if (p && onFloor(p)) {
					proposal.id = p.id;
					proposal.kind = p.kind;
					proposal.name = p.name;
					proposal.image = p.image;
					proposal.z = p.z;
					proposal.size = p.size;
					proposal.width = gesture.width;
					proposal.height = gesture.height;
					proposal.rotation = gesture.rotation;

					count = ghostOf(proposal, p.x, p.y, out, count);
				}
			}

			// And the thing placement is armed with, following the pointer.
			if (armed && pointer) {
				const shape = armedShape(armed, grid().cellSize);
				const point = snapFor(shape, pointer.x, pointer.y);
				count = ghostOf(shape, point.x, point.y, out, count);
			}

			out.length = count;

			return out;
		},

		marks(out) {
			out.length = 0;

			return deps.fog?.marks(out) ?? out;
		},

		inHand() {
			return deps.draw?.inHand() ?? null;
		},

		concealed,

		outlines(out) {
			let count = 0;

			const add = (o: Outline): void => {
				const slot = out[count] ?? (out[count] = blankOutline());
				Object.assign(slot, o);
				count++;
			};

			const cell = grid().cellSize;

			for (const id of selection.ids()) {
				const p = pawn(id);
				if (!p || !onFloor(p)) {
					continue;
				}

				// THE OUTLINE FOLLOWS THE PROPOSAL AND NOT THE PAWN, so a hand
				// on a corner handle sees the box it is about to get rather
				// than the one it started with.
				const at = shaped(p);
				const [halfW, halfH] = pawnExtents(at, cell);

				add({
					x: p.x, y: p.y, halfW, halfH,
					color: SELECT_COLOR, alpha: 0.95, thickness: 2,
					rect: p.kind === "object", rotation: at.rotation,
				});
			}

			// WHERE THE RULER WAS PUT DOWN. It is the end of the line the
			// pointer is NOT at, and the only one that needs marking -- a
			// fixed size on screen, because it is a control rather than
			// anything on the table.
			const line = measurement();
			if (line) {
				const radius = MEASURE_POINT * deps.scale();

				add({
					x: line.from.x, y: line.from.y, halfW: radius, halfH: radius,
					color: SELF_COLOR, alpha: 0.95, thickness: MEASURE_WIDTH,
					rect: false, rotation: 0,
				});
			}

			// A MARQUEE THAT HAS NOT OPENED IS NOT DRAWN, because until the
			// threshold is crossed it is a point: a rectangle of no width over
			// the spot somebody is clicking.
			if (gesture?.kind === "marquee" && gesture.moved) {
				const rect = marqueeRect(gesture);
				add({
					x: (rect.x1 + rect.x2) / 2,
					y: (rect.y1 + rect.y2) / 2,
					halfW: Math.abs(rect.x2 - rect.x1) / 2,
					halfH: Math.abs(rect.y2 - rect.y1) / 2,
					color: SELECT_COLOR, alpha: 0.8, thickness: 1, rect: true, rotation: 0,
				});
			}

			// AND THE FOG RECTANGLE BEING DRAGGED, which is the marquee's twin:
			// a box that follows the pointer and is gone when the button comes
			// up. It goes through this accessor rather than a pass of its own
			// because a rectangle is a rectangle, and the ring pass already
			// draws one.
			const fogBox = deps.fog?.outline();
			if (fogBox) {
				add(fogBox);
			}

			// AND THE ERASER'S RING, which is a cursor rather than a gesture:
			// it is on the table for as long as the mode is chosen and the
			// pointer is over it, because what it shows is where the eraser
			// reaches, and a reach nobody can see is a tool aimed by guessing.
			const eraser = deps.draw?.outline();
			if (eraser) {
				add(eraser);
			}

			out.length = count;

			return out;
		},

		rulers(out) {
			let count = 0;
			const g = grid();

			const add = (fromX: number, fromY: number, toX: number, toY: number, color: Ruler["color"]): void => {
				const a = cellAt(g, fromX, fromY);
				const b = cellAt(g, toX, toY);

				const slot = out[count] ?? (out[count] = { cells: [], x0: 0, y0: 0, x1: 0, y1: 0, label: "", color });
				supercover(a[0], a[1], b[0], b[1], slot.cells);

				const start = cellCentre(g, a[0], a[1]);
				const end = cellCentre(g, b[0], b[1]);

				slot.x0 = start[0];
				slot.y0 = start[1];
				slot.x1 = end[0];
				slot.y1 = end[1];
				slot.label = distanceLabel(cellsMoved(b[0] - a[0], b[1] - a[1], g.diagonals) * Math.max(0, g.feetPerCell));
				slot.color = color;

				count++;
			};

			// THE MEASURE TOOL'S RULER, AND IT IS THE ONE THAT HIGHLIGHTS NO
			// CELLS. Everything else here is a move -- a creature walking
			// through squares, scored by the rule the table plays under -- and
			// the tinted squares are how far that walk got. A free measurement
			// crosses squares without counting them, so painting them would be
			// the grid answering a question nobody asked it. The slot is a
			// shared one and may have come back from a drag with cells still in
			// it, which is what the truncation is for.
			const line = measurement();
			if (line) {
				const slot = out[count] ?? (out[count] = { cells: [], x0: 0, y0: 0, x1: 0, y1: 0, label: "", color: SELF_COLOR });

				slot.cells.length = 0;
				slot.x0 = line.from.x;
				slot.y0 = line.from.y;
				slot.x1 = line.to.x;
				slot.y1 = line.to.y;
				slot.label = distanceLabel(feetBetween(line.to.x - line.from.x, line.to.y - line.from.y, g));
				slot.color = SELF_COLOR;

				count++;
			}

			if (gesture?.kind === "drag") {
				const origin = gesture.origins.get(gesture.anchor);
				if (origin) {
					add(origin.x, origin.y, gesture.ghost.x, gesture.ghost.y, SELF_COLOR);
				}
			}

			for (const preview of previews.values()) {
				// THE ANCHOR IS THE FIRST POSITION, because that is the order
				// the server builds the list in -- selection() puts the anchor
				// at the head. It is a convention rather than a field, and the
				// one case it is wrong is a hidden wagon carrying a visible
				// rider, where a player's projected list starts with the rider.
				// The path is then drawn for the wrong pawn of the group, which
				// is a wrong line rather than a wrong move.
				const anchor = preview.positions[0];
				const p = anchor ? pawn(anchor.id) : null;
				if (anchor && p && onFloor(p)) {
					add(p.x, p.y, anchor.x, anchor.y, preview.color);
				}
			}

			out.length = count;

			return out;
		},

		handles(out) {
			return activeHandles(out);
		},

		bounds() {
			// THE SAME RULE THE OVERLAY DRAWS BY, or the label lands somewhere
			// other than the thing it is labelling. A group is boxed by the
			// whole selection; anything else is boxed by what is hovered, which
			// is null exactly when nothing is shown.
			const one = described();
			const ids = selection.size > 1 ? selection.ids() : one ? [one.id] : [];
			if (ids.length === 0) {
				return null;
			}

			const cell = grid().cellSize;
			let box: Rect | null = null;

			for (const id of ids) {
				const p = pawn(id);
				if (!p || !onFloor(p)) {
					continue;
				}

				// THE SCREEN-ALIGNED BOX AND NOT THE TOKEN'S OWN. What sits on
				// this is the DOM overlay, which is a rectangle on the page: a
				// long token turned on its side is wide here and tall in its
				// own frame, and an overlay placed on the second would land
				// across the middle of it.
				const [halfW, halfH] = boundsOf(shaped(p), cell);
				if (!box) {
					box = { x1: p.x - halfW, y1: p.y - halfH, x2: p.x + halfW, y2: p.y + halfH };

					continue;
				}

				box.x1 = Math.min(box.x1, p.x - halfW);
				box.y1 = Math.min(box.y1, p.y - halfH);
				box.x2 = Math.max(box.x2, p.x + halfW);
				box.y2 = Math.max(box.y2, p.y + halfH);
			}

			return box;
		},

		preview(event) {
			switch (event.type) {
				case "pawn.dragging": {
					if (!event.by || event.by === user) {
						return;
					}

					previews.set(event.by, {
						positions: event.pawns.map((at) => ({ id: at.id, x: at.x, y: at.y })),
						color: actorColor(event.by),
						at: performance.now(),
					});
					deps.invalidate();

					return;
				}

				// A COMMITTED MOVE IS WHAT ENDS A PREVIEW, which is why a
				// cancelled drag sends one rather than an event of its own.
				case "pawn.moved":
				case "pawn.updated":
				case "pawn.removed": {
					const touched = event.type === "pawn.moved"
						? event.pawns.map((at) => at.id)
						: [event.type === "pawn.updated" ? event.pawn.id : event.id];

					for (const [by, preview] of previews) {
						if (preview.positions.some((at) => touched.includes(at.id))) {
							previews.delete(by);
						}
					}

					// A pawn that left the table cannot stay selected: its next
					// drag would be answered not_found.
					const present = new Set(state.pawns.map((p) => p.id));
					selection.prune(present);
					if (hovered && !present.has(hovered)) {
						hovered = null;
					}

					// AND IT CANNOT STAY IN A HAND. A drag in progress that
					// still named it would be refused whole on release; the
					// anchor going is the gesture going, and a follower going
					// is one fewer follower.
					if (event.type === "pawn.removed" && gesture?.kind === "drag") {
						if (gesture.anchor === event.id) {
							gesture = null;
						} else {
							gesture.ids = gesture.ids.filter((id) => id !== event.id);
							gesture.origins.delete(event.id);
						}
					}

					// AND THE OVERLAY IS REWRITTEN WHATEVER CHANGED, because
					// what it says about a pawn -- its hit points, its
					// conditions, its name -- moved even when the selection did
					// not. These three are the committed events and arrive at
					// human pace; pawn.dragging, which does not, is handled
					// above and announces nothing.
					announce();

					return;
				}

				case "snapshot": {
					previews.clear();
					const present = new Set(state.pawns.map((p) => p.id));
					if (selection.prune(present)) {
						announce();
					}

					return;
				}
			}
		},

		floorChanged() {
			const present = new Set(state.pawns.filter(onFloor).map((p) => p.id));
			if (hovered && !present.has(hovered)) {
				hovered = null;
			}
			if (selection.prune(present)) {
				announce();
			}
		},

		arm,
		isArmed: () => armed !== null,

		onChange(fn) {
			changed = fn;
		},

		stop() {
			document.removeEventListener("keydown", onKeyDown);
		},
	};

	function past(from: Point, now: Point): boolean {
		// The threshold is in device pixels and the pointer is in CSS pixels,
		// which on a retina screen are not the same distance.
		const dpr = window.devicePixelRatio || 1;

		return Math.hypot(now.x - from.x, now.y - from.y) * dpr >= DRAG_THRESHOLD;
	}
}

// hitTest is what is under a point, topmost first.
//
// ON THE CPU, IN ONE PASS, WITH NO SORT. A few hundred pawns is a few
// microseconds, and the topmost is found by keeping the best rather than by
// ordering the whole table -- which would allocate on every pointer move.
//
// A DISC FOR A CREATURE AND A RECTANGLE FOR AN OBJECT, turned with the token,
// which is containsPoint and is exactly what the pawn pass drew: a click on the
// corner of a wagon's bounding box hits the wagon, a click on the corner of a
// goblin's does not hit the goblin, and a click just off the corner of a token
// turned forty degrees hits the table.
//
// TOPMOST IS compareStack AND NOT z. Every token is drawn under every creature,
// so clicking where a goblin stands on a rug picks the goblin -- the rug is not
// reachable there, because it is not what is on top there. It is the same
// comparison the draw order uses, which is the whole point: what a click picks
// is what a person can see.
export function hitTest(
	pawns: readonly Pawn[],
	layerID: string,
	grid: Grid,
	x: number,
	y: number,
	concealed?: (pawn: Pawn) => boolean,
): Pawn | null {
	let best: Pawn | null = null;

	for (const p of pawns) {
		if (p.layerId !== layerID) {
			continue;
		}

		// A CONCEALED PAWN IS SKIPPED RATHER THAN RETURNED AND DISCARDED, which
		// is the difference between "the fog eats the click" and "the click goes
		// through the fog". A goblin standing under the cover on top of a crate
		// the player CAN see must leave them the crate.
		if (concealed?.(p)) {
			continue;
		}
		if (best && compareStack(p, best) < 0) {
			continue;
		}
		if (containsPoint(p, x, y, grid.cellSize)) {
			best = p;
		}
	}

	return best;
}

// actorColor is a stable colour per player, from a hash of their id. Nothing is
// stored and nothing is sent: two clients compute the same answer because they
// are looking at the same ULID.
export function actorColor(id: string): readonly [number, number, number] {
	let hash = 0;
	for (let i = 0; i < id.length; i++) {
		hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
	}

	return ACTOR_COLORS[hash % ACTOR_COLORS.length] ?? ACTOR_COLORS[0];
}

// hexColor turns one of these palette entries into the #RRGGBB the protocol
// carries. It is here beside the palette rather than in draw.ts because what it
// converts is a colour this file owns, and the pen's default is that colour.
//
// SIX DIGITS AND NOT EIGHT. checkColor accepts both, and a stroke has no alpha:
// two overlapping segments of a translucent line blend twice and show a darker
// dot at every joint. See render/stroke-pass.ts.
export function hexColor(rgb: readonly [number, number, number]): string {
	let out = "#";
	for (const channel of rgb) {
		out += Math.round(Math.min(Math.max(channel, 0), 1) * 255)
			.toString(16)
			.padStart(2, "0");
	}

	return out.toUpperCase();
}

// expired drops previews nobody has updated. It is exported so the frame can
// call it without this module owning a timer -- a timer would keep a tab awake
// that has nothing to draw.
export function expirePreviews(previews: Map<string, { at: number }>, now: number): boolean {
	let dropped = false;

	for (const [by, preview] of previews) {
		if (now - preview.at > PREVIEW_TIMEOUT) {
			previews.delete(by);
			dropped = true;
		}
	}

	return dropped;
}

function blankDrawn(): Drawn {
	return {
		id: "", kind: "monster", name: "", image: "",
		x: 0, y: 0, z: 0, size: "medium",
		width: 0, height: 0, rotation: 0, hidden: false, health: null,
	};
}

// armedShape is the armed spec as something that can be snapped, measured and
// drawn. An object with no recorded picture size is one cell, which is what the
// hub places it at.
function armedShape(armed: Armed, cellSize: number): Ghostable {
	const cell = Math.max(1, cellSize);
	const object = armed.kind === "object";

	return {
		id: "armed",
		kind: armed.kind,
		name: armed.name,
		image: armed.image,
		x: 0,
		y: 0,
		z: 0,
		size: armed.size,
		width: object ? armed.width || cell : 0,
		height: object ? armed.height || cell : 0,

		// A TOKEN GOES DOWN SQUARE. There is no angle on the wire for a spawn
		// and none in the dialog; it is turned afterwards, with the handles.
		rotation: 0,
	};
}

// typing is whether a key went to a control rather than to the table.
//
// IT DUCK-TYPES RATHER THAN USING instanceof, and the reason is the test rig
// rather than taste: this module is exercised in Node, where HTMLElement does
// not exist -- so `target instanceof HTMLElement` is a ReferenceError rather
// than a false. What is actually being asked, does this thing take text, is
// answered by the two properties either way.
function blankOutline(): Outline {
	return {
		x: 0, y: 0, halfW: 0, halfH: 0,
		color: SELF_COLOR, alpha: 1, thickness: 1, rect: false, rotation: 0,
	};
}
