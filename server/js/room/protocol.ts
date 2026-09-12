export type ClearTrigger = "start" | "end";
export type ConditionColor = "blue" | "green" | "orange" | "pink" | "purple" | "red" | "white" | "yellow";
export type Diagonals = "equal" | "alternating";
export type FogMode = "reveal" | "hide";
export type GridLines = "off" | "solid" | "dashed";
export type HPBand = "healthy" | "bruised" | "bloody" | "veryBloody" | "nearDeath" | "dead";
export type InitiativeGrouping = "grouped" | "individual";
export type PawnKind = "player" | "monster" | "npc" | "object";
export type PawnLabels = "none" | "default" | "full";
export type Role = "gm" | "player";
export type ShapeKind = "rect" | "poly";
export type Size = "tiny" | "small" | "medium" | "large" | "huge" | "gargantuan";
export type Snap = "off" | "cells" | "halfCells";
export type StrokeKind = "free" | "rect" | "circle" | "cone";
export interface Condition {
	id: string;
	name: string;
	color: ConditionColor;
	duration: number;
	clear: ClearTrigger;
}
export interface FogShape {
	id: string;
	layerId: string;
	kind: ShapeKind;
	mode: FogMode;
	points: number[];
}
export interface Grid {
	lines: GridLines;
	cellSize: number;
	offsetX: number;
	offsetY: number;
	color: string;
	snap: Snap;
	feetPerCell: number;
	diagonals: Diagonals;
}
export interface Initiative {
	entries: InitiativeEntry[];
	active: string | null;
	round: number;
}
export interface InitiativeEntry {
	id: string;
	pawnIds: string[];
	name: string;
	initiative: number;
}
export interface Layer {
	id: string;
	name: string;
	map: MapRef | null;
	fogEnabled: boolean;
	fogPrefill: boolean;
}
export interface MapRef {
	assetId: string;
	gen: string;
	width: number;
	height: number;
	tileSize: number;
	maxZoom: number;
}
export interface Pawn {
	id: string;
	kind: PawnKind;
	layerId: string;
	name: string;
	image: string;
	x: number;
	y: number;
	z: number;
	size: Size;
	width: number;
	height: number;
	rotation: number;
	visible: boolean;
	hp: number | null;
	maxHp: number | null;
	hpBand: HPBand | null;
	ac: number | null;
	conditions: Condition[];
	ownerId: string | null;
	monsterId: string | null;
	characterId: string | null;
}
export interface PawnPosition {
	id: string;
	x: number;
	y: number;
}
export interface Player {
	id: string;
	name: string;
	avatar: string;
	characterId: string | null;
	characterName: string;
	role: Role;
	connected: boolean;
}
export interface RoomInfo {
	id: string;
	name: string;
	locked: boolean;
}
export interface SnapshotYou {
	id: string;
	role: Role;
}
export interface State {
	schema: number;
	seq: number;
	room: RoomInfo;
	table: Table;
	players: Player[];
	pawns: Pawn[];
	initiative: Initiative;
	fog: FogShape[];
	strokes: Stroke[];
}
export interface Stroke {
	id: string;
	by: string;
	layerId: string;
	kind: StrokeKind;
	color: string;
	width: number;
	points: number[];
	done: boolean;
}
export interface Table {
	layers: Layer[];
	activeLayer: string;
	grid: Grid;
	pawnLabels: PawnLabels;
	playersCanDraw: boolean;
	initiativeGrouping: InitiativeGrouping;
	fogPrefill: boolean;
}
export interface TableSettings {
	activeLayer: string;
	grid: Grid;
	pawnLabels: PawnLabels;
	playersCanDraw: boolean;
	initiativeGrouping: InitiativeGrouping;
	fogPrefill: boolean;
}
export interface FogAdd {
	type: "fog.add";
	cid: string;
	layer: string;
	kind: ShapeKind;
	mode: FogMode;
	points: number[];
}
export interface FogClear {
	type: "fog.clear";
	cid: string;
	layer: string;
}
export interface FogRemove {
	type: "fog.remove";
	cid: string;
	id: string;
}
export interface FogSetEnabled {
	type: "fog.setEnabled";
	cid: string;
	layer: string;
	enabled: boolean;
}
export interface FogSetPrefill {
	type: "fog.setPrefill";
	cid: string;
	layer: string;
	prefill: boolean;
}
export interface InitiativeActivate {
	type: "initiative.activate";
	cid: string;
	entry: string;
}
export interface InitiativeAdd {
	type: "initiative.add";
	cid: string;
	name: string;
	pawn: string | null;
}
export interface InitiativeClear {
	type: "initiative.clear";
	cid: string;
}
export interface InitiativeNext {
	type: "initiative.next";
	cid: string;
}
export interface InitiativeRemove {
	type: "initiative.remove";
	cid: string;
	entry: string;
}
export interface InitiativeReorder {
	type: "initiative.reorder";
	cid: string;
	ids: string[];
}
export interface InitiativeSet {
	type: "initiative.set";
	cid: string;
	entries: InitiativeEntry[];
	active: string | null;
}
export interface InitiativeSync {
	type: "initiative.sync";
	cid: string;
}
export interface PawnDrag {
	type: "pawn.drag";
	cid: string;
	anchor: string;
	x: number;
	y: number;
	others: string[];
}
export interface PawnMove {
	type: "pawn.move";
	cid: string;
	anchor: string;
	x: number;
	y: number;
	others: string[];
}
export interface PawnRemove {
	type: "pawn.remove";
	cid: string;
	ids: string[];
}
export interface PawnSetConditions {
	type: "pawn.setConditions";
	cid: string;
	id: string;
	conditions: Condition[];
}
export interface PawnSetLayer {
	type: "pawn.setLayer";
	cid: string;
	ids: string[];
	layer: string;
}
export interface PawnSetVisible {
	type: "pawn.setVisible";
	cid: string;
	ids: string[];
	visible: boolean;
}
export interface PawnSpawn {
	type: "pawn.spawn";
	cid: string;
	kind: PawnKind;
	layer: string;
	x: number;
	y: number;
	visible: boolean;
	monsterId?: string | null;
	characterId?: string | null;
	assetId?: string | null;
	name?: string;
	size?: Size;
	hp?: number | null;
	maxHp?: number | null;
	ac?: number | null;
}
export interface PawnSpawnCharacters {
	type: "pawn.spawnCharacters";
	cid: string;
}
export interface PawnUpdate {
	type: "pawn.update";
	cid: string;
	id: string;
	name?: string | null;
	hp?: number | null;
	maxHp?: number | null;
	ac?: number | null;
	size?: Size | null;
	z?: number | null;
	width?: number | null;
	height?: number | null;
	rotation?: number | null;
}
export interface Ping {
	type: "ping";
	cid: string;
	layer: string;
	x: number;
	y: number;
}
export interface PlayerKick {
	type: "player.kick";
	cid: string;
	id: string;
}
export interface StrokeBegin {
	type: "stroke.begin";
	cid: string;
	id: string;
	layer: string;
	kind: StrokeKind;
	color: string;
	width: number;
	points: number[];
}
export interface StrokeClear {
	type: "stroke.clear";
	cid: string;
	layer: string;
}
export interface StrokeEnd {
	type: "stroke.end";
	cid: string;
	id: string;
}
export interface StrokeErase {
	type: "stroke.erase";
	cid: string;
	ids: string[];
}
export interface StrokeExtend {
	type: "stroke.extend";
	cid: string;
	id: string;
	points: number[];
}
export interface SyncRequest {
	type: "sync.request";
	cid: string;
}
export interface TableAddLayer {
	type: "table.addLayer";
	cid: string;
	name: string;
}
export interface TableClear {
	type: "table.clear";
	cid: string;
}
export interface TableClearLayerMap {
	type: "table.clearLayerMap";
	cid: string;
	layer: string;
}
export interface TableMoveLayer {
	type: "table.moveLayer";
	cid: string;
	layer: string;
	index: number;
}
export interface TableRemoveLayer {
	type: "table.removeLayer";
	cid: string;
	layer: string;
}
export interface TableRenameLayer {
	type: "table.renameLayer";
	cid: string;
	layer: string;
	name: string;
}
export interface TableSetActiveLayer {
	type: "table.setActiveLayer";
	cid: string;
	layer: string;
}
export interface TableSetGrid {
	type: "table.setGrid";
	cid: string;
	grid: Grid;
}
export interface TableSetLayerMap {
	type: "table.setLayerMap";
	cid: string;
	layer: string;
	assetId: string;
}
export interface TableSetOptions {
	type: "table.setOptions";
	cid: string;
	pawnLabels: PawnLabels;
	playersCanDraw: boolean;
	initiativeGrouping: InitiativeGrouping;
	fogPrefill: boolean;
}
export type Command =
	| FogAdd
	| FogClear
	| FogRemove
	| FogSetEnabled
	| FogSetPrefill
	| InitiativeActivate
	| InitiativeAdd
	| InitiativeClear
	| InitiativeNext
	| InitiativeRemove
	| InitiativeReorder
	| InitiativeSet
	| InitiativeSync
	| PawnDrag
	| PawnMove
	| PawnRemove
	| PawnSetConditions
	| PawnSetLayer
	| PawnSetVisible
	| PawnSpawn
	| PawnSpawnCharacters
	| PawnUpdate
	| Ping
	| PlayerKick
	| StrokeBegin
	| StrokeClear
	| StrokeEnd
	| StrokeErase
	| StrokeExtend
	| SyncRequest
	| TableAddLayer
	| TableClear
	| TableClearLayerMap
	| TableMoveLayer
	| TableRemoveLayer
	| TableRenameLayer
	| TableSetActiveLayer
	| TableSetGrid
	| TableSetLayerMap
	| TableSetOptions
	;
export interface FogRemoved {
	type: "fog.removed";
	ids: string[];
}
export interface FogUpserted {
	type: "fog.upserted";
	shapes: FogShape[];
}
export interface InitiativeUpdated {
	type: "initiative.updated";
	initiative: Initiative;
}
export interface LayersUpdated {
	type: "layers.updated";
	layers: Layer[];
}
export interface PawnsMoved {
	type: "pawns.moved";
	pawns: PawnPosition[];
}
export interface PawnsRemoved {
	type: "pawns.removed";
	ids: string[];
}
export interface PawnsUpserted {
	type: "pawns.upserted";
	pawns: Pawn[];
}
export interface PlayersRemoved {
	type: "players.removed";
	ids: string[];
}
export interface PlayersUpserted {
	type: "players.upserted";
	players: Player[];
}
export interface RoomUpdated {
	type: "room.updated";
	room: RoomInfo;
}
export interface StrokeEnded {
	type: "strokes.ended";
	id: string;
}
export interface StrokeExtended {
	type: "strokes.extended";
	id: string;
	points: number[];
}
export interface StrokesRemoved {
	type: "strokes.removed";
	ids: string[];
}
export interface StrokesUpserted {
	type: "strokes.upserted";
	strokes: Stroke[];
}
export interface TableUpdated {
	type: "table.updated";
	table: TableSettings;
}
export type Change =
	| FogRemoved
	| FogUpserted
	| InitiativeUpdated
	| LayersUpdated
	| PawnsMoved
	| PawnsRemoved
	| PawnsUpserted
	| PlayersRemoved
	| PlayersUpserted
	| RoomUpdated
	| StrokeEnded
	| StrokeExtended
	| StrokesRemoved
	| StrokesUpserted
	| TableUpdated
	;
export interface Changes {
	type: "changes";
	seq: number;
	by?: string;
	events: Change[];
}
export interface ErrorEvent {
	type: "error";
	seq: number;
	by?: string;
	cid: string;
	code: string;
	heading: string;
	message: string;
}
export interface PawnDragging {
	type: "pawn.dragging";
	seq: number;
	by?: string;
	pawns: PawnPosition[];
}
export interface Pinged {
	type: "pinged";
	seq: number;
	by?: string;
	layer: string;
	x: number;
	y: number;
}
export interface PlayerKicked {
	type: "player.kicked";
	seq: number;
	by?: string;
	reason: string;
}
export interface RoomClosed {
	type: "room.closed";
	seq: number;
	by?: string;
}
export interface Snapshot {
	type: "snapshot";
	seq: number;
	by?: string;
	state: State;
	you: SnapshotYou;
	version: string;
}
export type Transient =
	| ErrorEvent
	| PawnDragging
	| Pinged
	| PlayerKicked
	| RoomClosed
	| Snapshot
	;
export type Frame =
	| Changes
	| ErrorEvent
	| PawnDragging
	| Pinged
	| PlayerKicked
	| RoomClosed
	| Snapshot
	;
export type Event = Change | Transient;
export const TRANSIENT_EVENTS: ReadonlySet<Frame["type"]> = new Set([
	"error",
	"pawn.dragging",
	"pinged",
	"player.kicked",
	"room.closed",
	"snapshot",
]);
