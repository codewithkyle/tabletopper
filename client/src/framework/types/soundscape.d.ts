interface ISound {
    ctx: AudioContext;
    gain: GainNode;
    buffer: AudioBuffer;
}
/**
 * @see https://material.io/design/sound/sound-resources.html
 * @license CC-BY-4.0
 */
declare class Soundscape {
    private sounds;
    private soundState;
    private hasTouched;
    private hasPointer;
    constructor();
    private addButtonListeners;
    private mousemove;
    private mouseleave;
    private mouseover;
    private focus;
    private click;
    private save;
    toggleSound(handle: string, isEnable: boolean): void;
    /**
     * Creates a new sound source.
     * Returns `null` if the sound does not exist OR if playback has been disabled.
     **/
    play(handle: string, loop?: boolean): AudioBufferSourceNode | null;
    /**
     * Pauses a sound source.
     */
    pause(handle: string): void;
    /**
     * Resumes a sound source.
     */
    resume(handle: string): void;
    setVolume(handle: string, volume: number): void;
    getVolume(handle: string): number;
    add(handle: string, src: string, force?: boolean): Promise<ISound>;
    get(handle: string): ISound | null;
    private createSound;
    load(): Promise<void>;
}
declare const sound: Soundscape;
export { sound as default };
