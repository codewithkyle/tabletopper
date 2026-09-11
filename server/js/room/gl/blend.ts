export function blended(gl: WebGL2RenderingContext, draw: () => void): void {
	gl.enable(gl.BLEND);
	gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
	draw();
	gl.disable(gl.BLEND);
}
