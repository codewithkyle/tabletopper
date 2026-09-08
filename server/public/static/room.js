var ir={"player.joined":"room:players","player.updated":"room:players","player.left":"room:players","initiative.updated":"room:initiative","room.updated":"room:info","table.updated":"room:tabletop"},or=["room:players","room:initiative","room:info","room:tabletop"],Xe="pawn:";function qe(e){if(e.type==="snapshot"){for(let r of or)window.dispatchEvent(new CustomEvent(r));return}switch(e.type){case"pawn.spawned":case"pawn.updated":Oe(e.pawn.id),window.dispatchEvent(new CustomEvent("window:retitle",{detail:{id:Xe+e.pawn.id,title:e.pawn.name}}));return;case"pawn.removed":Oe(e.id),window.dispatchEvent(new CustomEvent("window:close",{detail:{id:Xe+e.id}}));return}let t=ir[e.type];t&&window.dispatchEvent(new CustomEvent(t))}function Oe(e){window.dispatchEvent(new CustomEvent("room:pawn",{detail:{id:e}}))}function Ve(e,t){switch(t.type){case"snapshot":Object.assign(e,X(t.state));break;case"room.updated":e.room=X(t.room);break;case"table.updated":e.table=X(t.table);break;case"initiative.updated":e.initiative=X(t.initiative);break;case"player.joined":case"player.updated":he(e.players,X(t.player));break;case"player.left":e.players=_e(e.players,t.id);break;case"pawn.spawned":case"pawn.updated":he(e.pawns,X(t.pawn));break;case"pawn.removed":e.pawns=_e(e.pawns,t.id);break;case"fog.added":he(e.fog,X(t.shape));break;case"fog.removed":e.fog=_e(e.fog,t.id);break;case"stroke.began":he(e.strokes,X(t.stroke));break;case"stroke.ended":{let r=Re(e.strokes,t.id);r&&(r.done=!0);break}case"fog.cleared":e.fog=e.fog.filter(r=>r.layerId!==t.layer);break;case"stroke.cleared":e.strokes=e.strokes.filter(r=>r.layerId!==t.layer);break;case"stroke.erased":e.strokes=e.strokes.filter(r=>!t.ids.includes(r.id));break;case"pawn.moved":for(let r of t.pawns){let n=Re(e.pawns,r.id);n&&(n.x=r.x,n.y=r.y)}break;case"stroke.extended":{let r=Re(e.strokes,t.id);r&&r.points.push(...t.points);break}case"error":case"pinged":case"pawn.dragging":case"player.kicked":case"room.closed":return;default:{let r=t;throw new Error(`room: no reduction for ${r.type}`)}}ar(e)}function ar(e){e.players.sort(Ae),e.pawns.sort(Ae),e.strokes.sort(Ae)}function $e(){return{schema:0,seq:0,room:{id:"",name:"",locked:!1},table:{layers:[],activeLayer:"",grid:{visible:!0,cellSize:64,offsetX:0,offsetY:0,color:"#000000FF",snap:"cells",feetPerCell:5,diagonals:"equal"},monsterHp:"band",playersCanDraw:!0},players:[],pawns:[],initiative:{entries:[],active:null,round:0},fog:[],strokes:[]}}function he(e,t){let r=e.findIndex(n=>n.id===t.id);if(r===-1){e.push(t);return}e[r]=t}function _e(e,t){return e.filter(r=>r.id!==t)}function Re(e,t){return e.find(r=>r.id===t)}function Ae(e,t){return e.id<t.id?-1:e.id>t.id?1:0}function X(e){return structuredClone(e)}var Ke=new Set(["error","pawn.dragging","pinged","player.kicked","room.closed"]);var sr=500,cr=15e3,be=class{url;handlers;ws=null;timer;attempt=0;cid=0;seq=0;version="";ended=!1;resyncing=!1;constructor(t,r){this.url=t,this.handlers=r}start(){this.connect(),document.addEventListener("visibilitychange",()=>{document.visibilityState==="visible"&&!this.ws&&!this.ended&&this.reconnect(0)})}send(t){return this.cid+=1,this.raw(JSON.stringify({...t,cid:String(this.cid)}))}raw(t){return!this.ws||this.ws.readyState!==WebSocket.OPEN?!1:(this.ws.send(t),!0)}sequence(){return this.seq}build(){return this.version}connect(){this.handlers.status("connecting","");let t=new WebSocket(new URL(this.url,location.href));this.ws=t,t.addEventListener("open",()=>{this.attempt=0,this.handlers.status("open","")}),t.addEventListener("message",r=>{typeof r.data=="string"&&this.receive(r.data)}),t.addEventListener("close",r=>{if(this.ws=null,r.reason==="kicked"&&(this.ended=!0),this.ended){this.handlers.status("ended",r.reason);return}this.handlers.status("closed",r.reason),this.reconnect(this.backoff())})}receive(t){let r;try{r=JSON.parse(t)}catch{return}if(r.type==="snapshot"){if(this.seq=r.seq,this.resyncing=!1,this.version==="")this.version=r.version;else if(this.version!==r.version){location.reload();return}this.handlers.event(r);return}if(r.type==="room.closed"){this.ended=!0,this.handlers.event(r);return}if(Ke.has(r.type)){this.handlers.event(r);return}if(!(r.seq<=this.seq)){if(r.seq>this.seq+1){this.resync();return}this.seq=r.seq,this.handlers.event(r)}}resync(){this.resyncing||(this.resyncing=!0,this.send({type:"sync.request"}))}reconnect(t){window.clearTimeout(this.timer),this.timer=window.setTimeout(()=>this.connect(),t)}backoff(){let t=Math.min(cr,sr*2**this.attempt);return this.attempt+=1,t/2+Math.random()*(t/2)}};function je(e,t,r,n){let i=e.querySelector("[data-debug-connection]"),o=e.querySelector("[data-debug-seq]"),s=e.querySelector("[data-debug-version]"),a=e.querySelector("[data-debug-layer]"),u=e.querySelector("[data-debug-players]"),p=e.querySelector("[data-debug-events]"),c=e.querySelector("[data-debug-form]"),l=e.querySelector("[data-debug-input]"),d=e.querySelector("[data-debug-benchmark]"),f=e.querySelector("[data-debug-timing]"),m=0;d instanceof HTMLButtonElement&&n&&d.addEventListener("click",()=>{d.disabled=!0,f&&(f.textContent="sweeping..."),n.benchmark(b=>{d.disabled=!1,f&&(f.textContent=`avg ${b.average.toFixed(2)}ms  p95 ${b.p95.toFixed(2)}ms  ${b.frames} frames  ${b.tiles} tiles`)})});let y=e.querySelector("[data-debug-stress]");y instanceof HTMLButtonElement&&n&&y.addEventListener("click",()=>{let b=n.stress(m>0?0:500);m=b,y.textContent=b>0?`Stress (${b})`:"Stress"}),c instanceof HTMLFormElement&&l instanceof HTMLTextAreaElement&&c.addEventListener("submit",b=>{b.preventDefault();let v=l.value.trim();v!==""&&t.raw(v)&&(l.value="")});function h(){o&&(o.textContent=String(t.sequence())),s&&(s.textContent=t.build()),a&&(a.textContent=r.table.activeLayer),u&&u.replaceChildren(...r.players.map(b=>{let v=document.createElement("li");return v.textContent=`${b.name} (${b.role})${b.connected?"":" - away"}`,v}))}return{status(b,v){i&&(i.textContent=v===""?b:`${b} - ${v}`),h()},event(b){if(p){let v=document.createElement("li");for(v.textContent=JSON.stringify(b),p.prepend(v);p.childElementCount>20;)p.lastElementChild?.remove()}h()}}}var lr="alert:pending",dr="Removed from the room";function Ze(e){try{sessionStorage.setItem(lr,JSON.stringify({heading:dr,message:e}))}catch{}location.assign("/")}function Je(e,t,r){let n=document.querySelector("[data-layer-bar]");if(!(n instanceof HTMLElement))return null;let i=n.querySelector("[data-layer-select]");if(!(i instanceof HTMLSelectElement))return null;let o=n,s=i,a=o.querySelector("[data-layer-viewing]"),u=o.querySelector("[data-layer-activate]"),p=e.dataset.room??"",c="";s.addEventListener("change",()=>{r.view.choose(s.value),r.invalidate(),l()}),u?.addEventListener("click",()=>{let d=s.value;d===""||typeof htmx>"u"||htmx.ajax("POST",`/rooms/${p}/layers/${d}/activate`,{source:u})});function l(){let d=t.table.layers;if(o.hidden=d.length<2,o.hidden)return;let f=d.map(h=>`${h.id} ${h.name}`).join("");f!==c&&(c=f,s.replaceChildren(...d.map(h=>{let b=document.createElement("option");return b.value=h.id,b.textContent=h.name,b})));let m=r.view.viewed()?.id??"";s.value!==m&&(s.value=m);let y=r.view.following();a instanceof HTMLElement&&(a.hidden=y),u instanceof HTMLElement&&(u.hidden=y)}return l(),{refresh:l}}function tt(){return{x:0,y:0,zoom:1}}function rt(e){return Math.min(Math.max(e,.05),4)}function Qe(e,t,r,n,i){return i.x=(r-t.width/2)/e.zoom+e.x,i.y=(n-t.height/2)/e.zoom+e.y,i}function nt(e,t,r){let n=t.width/2/e.zoom,i=t.height/2/e.zoom;return r.x1=e.x-n,r.y1=e.y-i,r.x2=e.x+n,r.y2=e.y+i,r}function it(e,t,r){e.x-=t/e.zoom,e.y-=r/e.zoom}var Te={x:0,y:0},Se={x:0,y:0};function Z(e,t,r,n,i){let o=rt(e.zoom*i);o!==e.zoom&&(Qe(e,t,r,n,Te),e.zoom=o,Qe(e,t,r,n,Se),e.x+=Te.x-Se.x,e.y+=Te.y-Se.y)}function Pe(e,t,r){Z(e,t,t.width/2,t.height/2,r/e.zoom)}function Le(e,t,r,n){r<1||n<1||t.width<1||t.height<1||(e.zoom=rt(Math.min(t.width/r,t.height/n)*.9),e.x=r/2,e.y=n/2)}function ye(e,t,r,n){r<1||n<1||(e.x=et(e.x,t.width/e.zoom,r),e.y=et(e.y,t.height/e.zoom,n))}function et(e,t,r){let n=Math.min(t,r)/2;return Math.min(Math.max(e,n-t/2),r-n+t/2)}function O(e,t,r,n,i){let o=2*e.zoom*n/Math.max(t,1),s=-2*e.zoom*n/Math.max(r,1);return i[0]=o,i[1]=0,i[2]=0,i[3]=0,i[4]=s,i[5]=0,i[6]=-o*e.x,i[7]=-s*e.y,i[8]=1,i}function ot(e,t,r,n,i){let o=2*e.zoom*n/Math.max(t,1),s=-2*e.zoom*n/Math.max(r,1);return i[0]=1/o,i[1]=0,i[2]=0,i[3]=0,i[4]=1/s,i[5]=0,i[6]=e.x,i[7]=e.y,i[8]=1,i}var ur={alpha:!1,antialias:!1,depth:!1,stencil:!1,preserveDrawingBuffer:!1,powerPreference:"high-performance"};function st(e){return e.getContext("webgl2",ur)}function U(e,t,r){let n=at(e,e.VERTEX_SHADER,t),i=at(e,e.FRAGMENT_SHADER,r),o=e.createProgram();if(e.attachShader(o,n),e.attachShader(o,i),e.linkProgram(o),e.deleteShader(n),e.deleteShader(i),!e.getProgramParameter(o,e.LINK_STATUS)){let s=e.getProgramInfoLog(o);throw e.deleteProgram(o),new Error(`link failed: ${s??"no log"}`)}return o}function at(e,t,r){let n=e.createShader(t);if(!n)throw new Error("could not create a shader");if(e.shaderSource(n,r),e.compileShader(n),!e.getShaderParameter(n,e.COMPILE_STATUS)){let i=e.getShaderInfoLog(n);throw e.deleteShader(n),new Error(`compile failed: ${i??"no log"}`)}return n}function G(e,t,r){let n={};for(let i of r){let o=e.getUniformLocation(t,i);if(!o)throw new Error(`uniform ${i} is not active in this program`);n[i]=o}return n}function ct(e){let t=e.createVertexArray();e.bindVertexArray(t);let r=e.createBuffer();return e.bindBuffer(e.ARRAY_BUFFER,r),e.bufferData(e.ARRAY_BUFFER,new Float32Array([-1,-1,3,-1,-1,3]),e.STATIC_DRAW),e.enableVertexAttribArray(0),e.vertexAttribPointer(0,2,e.FLOAT,!1,0,0),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null),t}var Me="0123456789 ft.";function lt(e){let t=document.createElement("canvas").getContext("2d");if(!t)return null;let r="600 48px system-ui, sans-serif";t.font=r;let n=[...Me].map(l=>Math.ceil(t.measureText(l).width)),i=n.reduce((l,d)=>l+d+4,0),o=Math.ceil(48*1.35),s=document.createElement("canvas");s.width=Math.max(1,i),s.height=o;let a=s.getContext("2d");if(!a)return null;a.font=r,a.fillStyle="rgb(255 255 255)",a.textAlign="left",a.textBaseline="middle";let u=new Map,p=0;for(let l=0;l<Me.length;l++){let d=Me[l],f=n[l];a.fillText(d,p+2,o/2),u.set(d,{u0:(p+2)/s.width,v0:0,u1:(p+2+f)/s.width,v1:1,width:d===" "?0:f/o,advance:(f+2)/o}),p+=f+4}let c=e.createTexture();return e.bindTexture(e.TEXTURE_2D,c),e.texImage2D(e.TEXTURE_2D,0,e.RGBA8,s.width,s.height,0,e.RGBA,e.UNSIGNED_BYTE,s),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_MIN_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_MAG_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_WRAP_S,e.CLAMP_TO_EDGE),e.texParameteri(e.TEXTURE_2D,e.TEXTURE_WRAP_T,e.CLAMP_TO_EDGE),e.bindTexture(e.TEXTURE_2D,null),{texture:c,height:o,measure(l){let d=0;for(let f of l)d+=u.get(f)?.advance??0;return d},get:l=>u.get(l)??null,dispose(){e.deleteTexture(c)}}}var mr=`#version 300 es
layout(location = 0) in vec2 a_clip;

uniform mat3 u_clipToWorld;

out vec2 v_world;

void main() {
	v_world = (u_clipToWorld * vec3(a_clip, 1.0)).xy;
	gl_Position = vec4(a_clip, 0.0, 1.0);
}
`,fr=`#version 300 es
precision highp float;

in vec2 v_world;

uniform vec2 u_offset;
uniform float u_cell;
uniform vec4 u_color;

out vec4 outColor;

void main() {
	vec2 cells = (v_world - u_offset) / u_cell;
	vec2 perPixel = fwidth(cells);
	vec2 toEdge = abs(fract(cells - 0.5) - 0.5) / max(perPixel, vec2(1e-8));

	float line = 1.0 - clamp(min(toEdge.x, toEdge.y), 0.0, 1.0);
	float fade = smoothstep(2.0, 6.0, 1.0 / max(max(perPixel.x, perPixel.y), 1e-8));

	float alpha = u_color.a * line * fade;
	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(u_color.rgb, alpha);
}
`,pr=["u_clipToWorld","u_offset","u_cell","u_color"];function ut(e){let t=U(e,mr,fr),r=G(e,t,pr),n=ct(e),i=new Float32Array(9),o=new Float32Array(4);return{draw(s,a,u,p,c){!a.visible||a.cellSize<1||(hr(a.color,o),!(o[3]<=0)&&(e.useProgram(t),e.bindVertexArray(n),e.uniformMatrix3fv(r.u_clipToWorld,!1,ot(s,u,p,c,i)),e.uniform2f(r.u_offset,dt(a.offsetX,a.cellSize),dt(a.offsetY,a.cellSize)),e.uniform1f(r.u_cell,a.cellSize),e.uniform4f(r.u_color,o[0],o[1],o[2],o[3]),e.enable(e.BLEND),e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA),e.drawArrays(e.TRIANGLES,0,3),e.disable(e.BLEND),e.bindVertexArray(null)))},dispose(){e.deleteProgram(t),e.deleteVertexArray(n)}}}function dt(e,t){return(e%t+t)%t}function hr(e,t){t[0]=0,t[1]=0,t[2]=0,t[3]=1;let r=e.startsWith("#")?e.slice(1):e;return r.length!==6&&r.length!==8||!/^[0-9a-fA-F]+$/.test(r)||(t[0]=parseInt(r.slice(0,2),16)/255,t[1]=parseInt(r.slice(2,4),16)/255,t[2]=parseInt(r.slice(4,6),16)/255,r.length===8&&(t[3]=parseInt(r.slice(6,8),16)/255)),t}var ie=16,br=13,yr=10,vr=`#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_axis;
layout(location = 3) in vec4 a_color;
layout(location = 4) in vec4 a_uv;

uniform mat3 u_clip;

out vec2 v_uv;
flat out vec4 v_color;
flat out float v_textured;

void main() {
	vec2 world = a_rect.xy + a_corner.x * a_rect.zw + a_corner.y * a_axis.xy;

	v_uv = mix(a_uv.xy, a_uv.zw, a_corner);
	v_color = a_color;
	v_textured = a_axis.z;

	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`,xr=`#version 300 es
precision highp float;

in vec2 v_uv;
flat in vec4 v_color;
flat in float v_textured;

uniform sampler2D u_atlas;

out vec4 outColor;

void main() {
	float alpha = v_color.a;

	// The atlas is white on transparency, so its alpha IS the character's
	// coverage and the colour is whatever the caller asked for. That is what
	// lets one atlas serve a label per player, each in their own colour.
	if (v_textured > 0.5) {
		alpha *= texture(u_atlas, v_uv).a;
	}

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, alpha);
}
`,wr=["u_clip","u_atlas"];function Ie(e,t){let r=U(e,vr,xr),n=G(e,r,wr),i=e.createVertexArray();e.bindVertexArray(i);let o=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,o),e.bufferData(e.ARRAY_BUFFER,new Float32Array([0,0,1,0,0,1,1,1]),e.STATIC_DRAW),e.enableVertexAttribArray(0),e.vertexAttribPointer(0,2,e.FLOAT,!1,0,0);let s=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,s);let a=ie*4;for(let m=0;m<4;m++)e.enableVertexAttribArray(1+m),e.vertexAttribPointer(1+m,4,e.FLOAT,!1,a,m*16),e.vertexAttribDivisor(1+m,1);e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null);let u=new Float32Array(128*ie),p=0,c=1,l=new Float32Array(9),d=e.createTexture();e.bindTexture(e.TEXTURE_2D,d),e.texImage2D(e.TEXTURE_2D,0,e.RGBA8,1,1,0,e.RGBA,e.UNSIGNED_BYTE,new Uint8Array([255,255,255,255])),e.bindTexture(e.TEXTURE_2D,null);function f(m,y,h,b,v,R,M,T,g,w,E,S,I){let P=(p+1)*ie;if(P>u.length){let C=u.length;for(;C<P;)C*=2;let H=new Float32Array(C);H.set(u),u=H}let x=p*ie;u[x]=m,u[x+1]=y,u[x+2]=h,u[x+3]=b,u[x+4]=v,u[x+5]=R,u[x+6]=M,u[x+7]=0,u[x+8]=T[0],u[x+9]=T[1],u[x+10]=T[2],u[x+11]=g,u[x+12]=w,u[x+13]=E,u[x+14]=S,u[x+15]=I,p++}return{begin(m){p=0,c=m},cell(m,y,h,b,v){f(m,y,h,0,0,h,0,b,v,0,0,0,0)},line(m,y,h,b,v,R,M){let T=h-m,g=b-y,w=Math.hypot(T,g);if(w<=0)return;let E=v*c/2,S=-g/w*E,I=T/w*E;f(m-S,y-I,T,g,S*2,I*2,0,R,M,0,0,0,0)},label(m,y,h,b,v){if(!t)return;let R=br*c,M=t.measure(m)*R,T=y-M/2,g=h-yr*c-R;for(let w of m){let E=t.get(w);if(E){if(E.width>0){let S=E.width*R;f(T,g,S,0,0,R,1,b,v,E.u0,E.v0,E.u1,E.v1)}T+=E.advance*R}}},draw(m,y,h,b){p!==0&&(e.useProgram(r),e.bindVertexArray(i),e.bindBuffer(e.ARRAY_BUFFER,s),e.bufferData(e.ARRAY_BUFFER,u.subarray(0,p*ie),e.DYNAMIC_DRAW),e.activeTexture(e.TEXTURE0),e.bindTexture(e.TEXTURE_2D,t?t.texture:d),e.uniform1i(n.u_atlas,0),e.uniformMatrix3fv(n.u_clip,!1,O(m,y,h,b,l)),e.enable(e.BLEND),e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA),e.drawArraysInstanced(e.TRIANGLE_STRIP,0,4,p),e.disable(e.BLEND),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null))},dispose(){e.deleteProgram(r),e.deleteVertexArray(i),e.deleteBuffer(o),e.deleteBuffer(s),e.deleteTexture(d)}}}function oe(e,t){if(e<=0)return 0;if(t>=31)return 1;let r=1<<t;return r>=e?1:e+r-1>>t}function Ce(e,t,r){return t<1?0:Math.ceil(oe(e,r)/t)}var ft=96,mt=8,gr=4;function pt(e,t,r){let n=e*t;return n>0?Math.min(Math.max(Math.round(Math.log2(1/n)),0),r):r}function J(e,t){let r=oe(e,t);return r>0?e/r:1}function ht(){return{x0:0,y0:0,x1:-1,y1:-1}}function bt(e){return Math.max(0,e.x1-e.x0+1)*Math.max(0,e.y1-e.y0+1)}function yt(e,t,r,n){n.x0=0,n.y0=0,n.x1=-1,n.y1=-1;let i=Ce(e.width,e.tileSize,t),o=Ce(e.height,e.tileSize,t);if(i<1||o<1)return n;let s=e.tileSize*J(e.width,t),a=e.tileSize*J(e.height,t),u=Math.max(r.x1,0),p=Math.min(r.x2,e.width),c=Math.max(r.y1,0),l=Math.min(r.y2,e.height);return p<=u||l<=c||(n.x0=ve(Math.floor(u/s),0,i-1),n.x1=ve(Math.ceil(p/s)-1,0,i-1),n.y0=ve(Math.floor(c/a),0,o-1),n.y1=ve(Math.ceil(l/a)-1,0,o-1)),n}function vt(e,t,r,n,i){let o=J(e.width,t),s=J(e.height,t),a=oe(e.width,t),u=oe(e.height,t);return i.x1=r*e.tileSize*o,i.y1=n*e.tileSize*s,i.x2=Math.min((r+1)*e.tileSize,a)*o,i.y2=Math.min((n+1)*e.tileSize,u)*s,i}function ke(e,t,r,n,i,o){let s=J(e.width,t),a=J(e.height,t);return o.x1=(i.x1/s-r*e.tileSize)/e.tileSize,o.y1=(i.y1/a-n*e.tileSize)/e.tileSize,o.x2=(i.x2/s-r*e.tileSize)/e.tileSize,o.y2=(i.y2/a-n*e.tileSize)/e.tileSize,o}function De(e,t,r,n){return`${e.assetId}:${e.gen}:${t}:${r}:${n}`}function xt(e){return`${e.assetId}:${e.gen}:`}function wt(e,t,r,n){return`/assets/maps/${e.assetId}/tiles/${e.gen}/${t}/${r}_${n}.webp`}var Q=class{byKey=new Map;free=[];frame=0;capacity;constructor(t){this.capacity=t;for(let r=t-1;r>=0;r--)this.free.push(r)}tick(){this.frame++}get(t){let r=this.byKey.get(t);return r&&(r.used=this.frame),r}has(t){return this.byKey.has(t)}claim(t,r,n){let i=this.byKey.get(t);if(i)return i.w=r,i.h=n,i.used=this.frame,i;let o=this.free.pop();if(o===void 0&&(o=this.evict()),o===void 0)return null;let s={layer:o,w:r,h:n,used:this.frame};return this.byKey.set(t,s),s}evict(){let t,r=1/0;for(let[i,o]of this.byKey)o.used!==this.frame&&o.used<r&&(r=o.used,t=i);if(t===void 0)return;let n=this.byKey.get(t);return this.byKey.delete(t),n?.layer}get size(){return this.byKey.size}};function Er(e){return createImageBitmap(e,{premultiplyAlpha:"none",colorSpaceConversion:"none"})}function xe(e,t=Er){let r=[],n=new Map,i=[],o=new Set,s=new Set,a=0,u=!1;function p(c){let l=new AbortController;n.set(c.key,l),a++,fetch(c.url,{credentials:"same-origin",signal:l.signal}).then(d=>d.status===404?(o.add(c.key),null):d.ok?d.blob():null).then(d=>d?t(d):null).then(d=>{d&&(i.push({key:c.key,bitmap:d}),e())}).catch(()=>{}).finally(()=>{n.delete(c.key)})}return{begin(){r.length=0,s.clear()},want(c,l,d){s.add(c),!(o.has(c)||n.has(c))&&r.push({key:c,url:l,priority:d})},end(){if(u||r.length===0)return;r.sort((l,d)=>l.priority-d.priority);let c=mt-n.size;for(let[l,d]of n){if(c>=r.length)break;s.has(l)||(d.abort(),n.delete(l),c++)}for(let l of r){if(n.size>=mt)break;n.has(l.key)||p(l)}},drain(c){let l=Math.min(i.length,gr);for(let d=0;d<l;d++){let f=i[d];c(f.key,f.bitmap),f.bitmap.close()}return i.splice(0,l),i.length},abandon(c){for(let[l,d]of n)l.startsWith(c)&&d.abort();for(let l=i.length-1;l>=0;l--)i[l].key.startsWith(c)&&(i[l].bitmap.close(),i.splice(l,1))},fetched:()=>a,stop(){u=!0;for(let c of n.values())c.abort();n.clear();for(let c of i)c.bitmap.close();i.length=0}}}function ve(e,t,r){return Math.min(Math.max(e,t),r)}var F=256,_r=128,$={player:[.29,.55,.9],monster:[.82,.28,.28],npc:[.33,.67,.44],object:[.55,.51,.46]},Fe={red:[.94,.27,.27],orange:[.98,.57,.24],yellow:[.98,.83,.25],green:[.3,.76,.42],blue:[.3,.6,.96],purple:[.65,.4,.94],pink:[.96,.5,.75],white:[.95,.95,.95]},ze="skull";function gt(e,t){let r=Math.min(_r,e.getParameter(e.MAX_ARRAY_TEXTURE_LAYERS)),n=e.createTexture();e.bindTexture(e.TEXTURE_2D_ARRAY,n),e.texStorage3D(e.TEXTURE_2D_ARRAY,1,e.RGBA8,F,F,r),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_MIN_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_MAG_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_WRAP_S,e.CLAMP_TO_EDGE),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_WRAP_T,e.CLAMP_TO_EDGE),e.bindTexture(e.TEXTURE_2D_ARRAY,null);let i=new Q(r),o=xe(t,Rr),s=new Map,a=new Set,u=0;function p(l,d,f,m){let y=i.claim(l,f,m);return y?(e.bindTexture(e.TEXTURE_2D_ARRAY,n),e.texSubImage3D(e.TEXTURE_2D_ARRAY,0,0,0,y.layer,f,m,1,e.RGBA,e.UNSIGNED_BYTE,d),e.bindTexture(e.TEXTURE_2D_ARRAY,null),u++,y):null}function c(l,d){a.add(l);let f=i.get(l);if(f)return f;s.set(l,d);let m=d();return m?p(l,m,m.width,m.height):null}return{begin(l){if(i.tick(),l)a.clear();else for(let d of a)i.get(d);o.begin()},sprite(l,d){if(l==="")return null;a.add(l);let f=i.get(l);return f||(o.want(l,l,d),null)},initials(l,d){let f=Ar(d);return c(`initials:${l}:${f}`,()=>Tr(l,f))},glyph(l){return c(`glyph:${l}`,()=>l===ze?Sr():null)},end(){o.end();let l=0,d=o.drain((f,m)=>{l++,p(f,m,m.width,m.height)});return l>0||d>0},texture:()=>n,epoch:()=>u,dispose(){o.stop(),s.clear(),a.clear(),e.deleteTexture(n)}}}async function Rr(e){let t={premultiplyAlpha:"none",colorSpaceConversion:"none"},r=await createImageBitmap(e,t),n=Math.max(r.width,r.height);if(n<=F||n===0)return r;let i=F/n,o=await createImageBitmap(r,{...t,resizeWidth:Math.max(1,Math.round(r.width*i)),resizeHeight:Math.max(1,Math.round(r.height*i)),resizeQuality:"high"});return r.close(),o}function Ar(e){let t=e.trim().split(/\s+/).filter(r=>r!=="");return t.length===0?"?":t.length===1?t[0].slice(0,2).toUpperCase():(t[0][0]+t[1][0]).toUpperCase()}function Tr(e,t){let r=Et();if(!r)return null;let[n,i,o]=$[e]??$.npc;return r.fillStyle=`rgb(${n*255} ${i*255} ${o*255})`,r.fillRect(0,0,F,F),r.fillStyle="rgb(255 255 255)",r.font=`600 ${t.length>1?104:140}px system-ui, sans-serif`,r.textAlign="center",r.textBaseline="middle",r.fillText(t,F/2,F/2+4),r.canvas}function Sr(){let e=Et();return e?(e.font=`${Math.round(F*.8)}px system-ui, sans-serif`,e.textAlign="center",e.textBaseline="middle",e.fillText("\u{1F480}",F/2,F/2),e.canvas):null}function Et(){let e=document.createElement("canvas");return e.width=F,e.height=F,e.getContext("2d")}function _t(e){if(e.kind==="object")return[Math.max(1,e.footprintW),Math.max(1,e.footprintH)];switch(e.size){case"large":return[2,2];case"huge":return[3,3];case"gargantuan":return[4,4];default:return[1,1]}}var ae=16,Pr=2,Lr=.6,Mr=.7,Ir=.5,Cr=.7,we=256,kr=`#version 300 es
#define BORDER_PIXELS ${Pr}.0

layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_border;
layout(location = 3) in vec4 a_style;
layout(location = 4) in vec4 a_fit;

uniform mat3 u_clip;
uniform float u_scale;

out vec2 v_local;
flat out vec4 v_border;
flat out vec4 v_style;
flat out vec4 v_fit;
flat out float v_edge;

void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_border = a_border;
	v_style = a_style;
	v_fit = a_fit;

	// The border is a device-pixel width turned into local units, which is why
	// it is computed here and not in the fragment shader: the half extent is a
	// per-instance value and this is the last place it is one.
	v_edge = BORDER_PIXELS / max(a_rect.z * u_scale, 1e-4);

	vec2 world = a_rect.xy + v_local * a_rect.zw;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`,Dr=`#version 300 es
precision highp float;
precision highp sampler2DArray;

in vec2 v_local;
flat in vec4 v_border;
flat in vec4 v_style;
flat in vec4 v_fit;
flat in float v_edge;

uniform sampler2DArray u_sprites;

out vec4 outColor;

void main() {
	float layer = v_style.x;
	float shape = v_style.y;
	float alpha = v_style.z;
	float grey  = v_style.w;

	// The picture, fitted. v_fit.xy is how much of the quad the image covers on
	// each axis -- under one for a letterboxed object, over one for a cropped
	// creature -- and v_fit.zw is how much of the 256 square layer the image
	// actually occupies, which is short of 1 for anything that is not square.
	vec2 t = (v_local / v_fit.xy) * 0.5 + 0.5;

	vec4 picture = vec4(0.0);
	if (layer >= 0.0 && t.x >= 0.0 && t.y >= 0.0 && t.x <= 1.0 && t.y <= 1.0) {
		picture = texture(u_sprites, vec3(t * v_fit.zw, layer));
	}

	vec3 rgb;
	float cover;

	if (shape < 0.5) {
		float r = length(v_local);
		float aa = max(fwidth(r), 1e-5);

		cover = 1.0 - smoothstep(1.0 - aa, 1.0, r);
		if (cover <= 0.0) {
			discard;
		}

		// WHATEVER THE PICTURE DOES NOT COVER IS THE KIND'S COLOUR, so a token
		// saved with a transparent background reads as a pawn rather than as a
		// hole in the table with a ring round it.
		rgb = mix(v_border.rgb, picture.rgb, picture.a);

		float border = smoothstep(1.0 - v_edge - aa, 1.0 - v_edge, r);
		rgb = mix(rgb, v_border.rgb, border * v_border.a);
	} else {
		cover = picture.a;
		if (cover <= 0.0) {
			discard;
		}
		rgb = picture.rgb;
	}

	// Desaturation is the GM's marker for a pawn players cannot see, and the
	// whole of what a dead creature is drawn as under its skull.
	rgb = mix(rgb, vec3(dot(rgb, vec3(0.299, 0.587, 0.114))), grey);

	outColor = vec4(rgb, cover * alpha);
}
`,Fr=["u_clip","u_sprites","u_scale"];function At(e){let t=U(e,kr,Dr),r=G(e,t,Fr),n=e.createVertexArray();e.bindVertexArray(n);let i=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,i),e.bufferData(e.ARRAY_BUFFER,new Float32Array([0,0,1,0,0,1,1,1]),e.STATIC_DRAW),e.enableVertexAttribArray(0),e.vertexAttribPointer(0,2,e.FLOAT,!1,0,0);let o=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,o);let s=ae*4;for(let m=0;m<4;m++)e.enableVertexAttribArray(1+m),e.vertexAttribPointer(1+m,4,e.FLOAT,!1,s,m*16),e.vertexAttribDivisor(1+m,1);e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null);let a=new Float32Array(64*ae),u=0,p=null,c=[],l=new Float32Array(9);function d(m){let y=m*ae;if(y<=a.length)return;let h=a.length;for(;h<y;)h*=2;a=new Float32Array(h)}function f(m,y,h,b,v,R,M,T,g,w,E,S,I,P){let x=u*ae;a[x]=m,a[x+1]=y,a[x+2]=h,a[x+3]=b,a[x+4]=v[0],a[x+5]=v[1],a[x+6]=v[2],a[x+7]=R,a[x+8]=M,a[x+9]=T,a[x+10]=g,a[x+11]=w,a[x+12]=E,a[x+13]=S,a[x+14]=I,a[x+15]=P,u++}return{build(m,y,h){if(u=0,p=h.texture(),d(m.length*2),c.length!==m.length)c=m.map((b,v)=>v);else for(let b=0;b<m.length;b++)c[b]=b;c.sort((b,v)=>m[b].z-m[v].z||(m[b].id<m[v].id?-1:1));for(let b of c){let v=m[b],[R,M]=Be(v,y.cellSize),T=v.kind==="object",g=v.hidden?Lr:1,w=v.hidden?Mr:v.dead?1:0,E=h.sprite(v.image,0)??h.initials(v.kind,v.name),S=E?E.layer:-1,[I,P]=E?Rt(E.w,E.h,R,M,!T):[1,1];if(f(v.x,v.y,R,M,$[v.kind]??$.npc,T?0:1,S,T?1:0,g,w,I,P,E?E.w/we:1,E?E.h/we:1),v.dead&&!T){let x=h.glyph(ze);if(x){let C=R*Cr,[H,D]=Rt(x.w,x.h,C,C,!1);f(v.x,v.y,C,C,$[v.kind]??$.npc,0,x.layer,1,g,0,H,D,x.w/we,x.h/we)}}}},draw(m,y,h,b){u===0||!p||(e.useProgram(t),e.bindVertexArray(n),e.bindBuffer(e.ARRAY_BUFFER,o),e.bufferData(e.ARRAY_BUFFER,a.subarray(0,u*ae),e.DYNAMIC_DRAW),e.activeTexture(e.TEXTURE0),e.bindTexture(e.TEXTURE_2D_ARRAY,p),e.uniform1i(r.u_sprites,0),e.uniform1f(r.u_scale,m.zoom*b),e.uniformMatrix3fv(r.u_clip,!1,O(m,y,h,b,l)),e.enable(e.BLEND),e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA),e.drawArraysInstanced(e.TRIANGLE_STRIP,0,4,u),e.disable(e.BLEND),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null))},dispose(){e.deleteProgram(t),e.deleteVertexArray(n),e.deleteBuffer(i),e.deleteBuffer(o)}}}function Be(e,t){let r=Math.max(1,t),[n,i]=_t(e);if(e.kind==="object")return[n*r/2,i*r/2];let o=e.size==="tiny"?Ir:1;return[n*r*o/2,i*r*o/2]}function Rt(e,t,r,n,i){if(e<=0||t<=0||r<=0||n<=0)return[1,1];let o=e/t,s=r/n,a=i?Math.max:Math.min;return[a(1,o/s),a(1,s/o)]}var se=12,Tt=0;var zr=`#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_color;
layout(location = 3) in vec4 a_style;

uniform mat3 u_clip;
uniform float u_scale;

out vec2 v_local;
flat out vec4 v_color;
flat out vec2 v_half;
flat out vec2 v_style;

void main() {
	v_local = a_corner * 2.0 - 1.0;
	v_color = a_color;
	v_half = a_rect.zw;

	// The thickness arrives in device pixels and leaves in map pixels, which is
	// the one conversion this pass exists to get right.
	v_style = vec2(a_style.x / max(u_scale, 1e-4), a_style.y);

	// The quad is grown by the line's own width so a ring drawn exactly at the
	// pawn's radius has its outer half somewhere to be rasterised. Without it
	// the outer edge is clipped by the quad and every ring reads as thinner
	// than the one before it.
	vec2 grown = a_rect.zw + v_style.x;
	vec2 world = a_rect.xy + v_local * grown;

	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
	v_local *= grown / max(a_rect.zw, vec2(1e-4));
}
`,Br=`#version 300 es
precision highp float;

in vec2 v_local;
flat in vec4 v_color;
flat in vec2 v_half;
flat in vec2 v_style;

out vec4 outColor;

void main() {
	float thickness = v_style.x;

	// inside is how far this fragment is INSIDE the shape's edge, in map
	// pixels. Negative is outside it.
	float inside;
	if (v_style.y < 0.5) {
		// An ellipse's edge distance is only exact for a circle, which every
		// ring in this app is -- a creature's quad is square. The approximation
		// is measured along the radius, which for a circle IS the distance.
		float r = length(v_local);
		inside = (1.0 - r) * v_half.x;
	} else {
		vec2 edge = (vec2(1.0) - abs(v_local)) * v_half;
		inside = min(edge.x, edge.y);
	}

	// The line straddles the edge, half in and half out, so a ring at a pawn's
	// radius touches the pawn rather than sitting a line's width inside it.
	float aa = max(fwidth(inside), 1e-5);
	float alpha = smoothstep(-thickness * 0.5 - aa, -thickness * 0.5, inside)
		* (1.0 - smoothstep(thickness * 0.5, thickness * 0.5 + aa, inside));

	if (alpha <= 0.0) {
		discard;
	}

	outColor = vec4(v_color.rgb, v_color.a * alpha);
}
`,Nr=["u_clip","u_scale"];function St(e){let t=U(e,zr,Br),r=G(e,t,Nr),n=e.createVertexArray();e.bindVertexArray(n);let i=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,i),e.bufferData(e.ARRAY_BUFFER,new Float32Array([0,0,1,0,0,1,1,1]),e.STATIC_DRAW),e.enableVertexAttribArray(0),e.vertexAttribPointer(0,2,e.FLOAT,!1,0,0);let o=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,o);let s=se*4;for(let c=0;c<3;c++)e.enableVertexAttribArray(1+c),e.vertexAttribPointer(1+c,4,e.FLOAT,!1,s,c*16),e.vertexAttribDivisor(1+c,1);e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null);let a=new Float32Array(64*se),u=0,p=new Float32Array(9);return{begin(){u=0},add(c,l,d,f,m,y,h,b){let v=(u+1)*se;if(v>a.length){let M=a.length;for(;M<v;)M*=2;let T=new Float32Array(M);T.set(a),a=T}let R=u*se;a[R]=c,a[R+1]=l,a[R+2]=d,a[R+3]=f,a[R+4]=m[0],a[R+5]=m[1],a[R+6]=m[2],a[R+7]=y,a[R+8]=h,a[R+9]=b,a[R+10]=0,a[R+11]=0,u++},draw(c,l,d,f){u!==0&&(e.useProgram(t),e.bindVertexArray(n),e.bindBuffer(e.ARRAY_BUFFER,o),e.bufferData(e.ARRAY_BUFFER,a.subarray(0,u*se),e.DYNAMIC_DRAW),e.uniform1f(r.u_scale,c.zoom*f),e.uniformMatrix3fv(r.u_clip,!1,O(c,l,d,f,p)),e.enable(e.BLEND),e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA),e.drawArraysInstanced(e.TRIANGLE_STRIP,0,4,u),e.disable(e.BLEND),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null))},dispose(){e.deleteProgram(t),e.deleteVertexArray(n),e.deleteBuffer(i),e.deleteBuffer(o)}}}var ce=9,Hr=`#version 300 es
layout(location = 0) in vec2 a_corner;
layout(location = 1) in vec4 a_rect;
layout(location = 2) in vec4 a_uv;
layout(location = 3) in float a_layer;

uniform mat3 u_clip;

out vec2 v_uv;
flat out float v_layer;

void main() {
	vec2 world = a_rect.xy + a_corner * a_rect.zw;
	v_uv = mix(a_uv.xy, a_uv.zw, a_corner);
	v_layer = a_layer;
	gl_Position = vec4((u_clip * vec3(world, 1.0)).xy, 0.0, 1.0);
}
`,Ur=`#version 300 es
precision highp float;
precision highp sampler2DArray;

in vec2 v_uv;
flat in float v_layer;

uniform sampler2DArray u_tiles;
uniform float u_alpha;

out vec4 outColor;

void main() {
	vec4 texel = texture(u_tiles, vec3(v_uv, v_layer));
	outColor = vec4(texel.rgb, texel.a * u_alpha);
}
`,Gr=["u_clip","u_tiles","u_alpha"];function Pt(e,t){let r=U(e,Hr,Ur),n=G(e,r,Gr),i=Math.min(ft,e.getParameter(e.MAX_ARRAY_TEXTURE_LAYERS)),o=new Map,s=new Map,a=xe(t),u=e.createVertexArray();e.bindVertexArray(u);let p=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,p),e.bufferData(e.ARRAY_BUFFER,new Float32Array([0,0,1,0,0,1,1,1]),e.STATIC_DRAW),e.enableVertexAttribArray(0),e.vertexAttribPointer(0,2,e.FLOAT,!1,0,0);let c=e.createBuffer();e.bindBuffer(e.ARRAY_BUFFER,c);let l=ce*4;e.enableVertexAttribArray(1),e.vertexAttribPointer(1,4,e.FLOAT,!1,l,0),e.vertexAttribDivisor(1,1),e.enableVertexAttribArray(2),e.vertexAttribPointer(2,4,e.FLOAT,!1,l,16),e.vertexAttribDivisor(2,1),e.enableVertexAttribArray(3),e.vertexAttribPointer(3,1,e.FLOAT,!1,l,32),e.vertexAttribDivisor(3,1),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null);let d=new Float32Array(256*ce),f=0,m=new Float32Array(9),y={x1:0,y1:0,x2:0,y2:0},h={x1:0,y1:0,x2:0,y2:0},b={x1:0,y1:0,x2:0,y2:0},v=ht();function R(g){let w=o.get(g);if(w)return w;let E=e.createTexture();e.bindTexture(e.TEXTURE_2D_ARRAY,E),e.texStorage3D(e.TEXTURE_2D_ARRAY,1,e.RGBA8,g,g,i),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_MIN_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_MAG_FILTER,e.LINEAR),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_WRAP_S,e.CLAMP_TO_EDGE),e.texParameteri(e.TEXTURE_2D_ARRAY,e.TEXTURE_WRAP_T,e.CLAMP_TO_EDGE),e.bindTexture(e.TEXTURE_2D_ARRAY,null);let S={texture:E,slots:new Q(i),tileSize:g};return o.set(g,S),S}function M(g){let w=g*ce;if(w<=d.length)return;let E=d.length;for(;E<w;)E*=2;d=new Float32Array(E)}function T(g){let w=f*ce;d[w]=h.x1,d[w+1]=h.y1,d[w+2]=h.x2-h.x1,d[w+3]=h.y2-h.y1,d[w+4]=b.x1,d[w+5]=b.y1,d[w+6]=b.x2,d[w+7]=b.y2,d[w+8]=g.layer,f++}return{begin(){for(let g of o.values())g.slots.tick();a.begin()},draw(g,w,E,S,I,P){if(w.width<1||w.height<1||w.tileSize<1)return;let x=R(w.tileSize),C=pt(g.zoom,P,w.maxZoom);nt(g,{width:S/P,height:I/P},y),yt(w,C,y,v);let H=bt(v);if(H!==0){M(H),f=0;for(let D=v.y0;D<=v.y1;D++)for(let L=v.x0;L<=v.x1;L++){vt(w,C,L,D,h);let re=De(w,C,L,D),ue=x.slots.get(re);if(ue){ke(w,C,L,D,h,b),T(ue);continue}s.set(re,w.tileSize),a.want(re,wt(w,C,L,D),Math.hypot((h.x1+h.x2)/2-g.x,(h.y1+h.y2)/2-g.y));for(let V=C+1;V<=w.maxZoom;V++){let me=V-C,K=L>>me,ne=D>>me,j=x.slots.get(De(w,V,K,ne));if(j){ke(w,V,K,ne,h,b),T(j);break}}}f!==0&&(e.useProgram(r),e.bindVertexArray(u),e.bindBuffer(e.ARRAY_BUFFER,c),e.bufferData(e.ARRAY_BUFFER,d.subarray(0,f*ce),e.DYNAMIC_DRAW),e.activeTexture(e.TEXTURE0),e.bindTexture(e.TEXTURE_2D_ARRAY,x.texture),e.uniform1i(n.u_tiles,0),e.uniform1f(n.u_alpha,E),e.uniformMatrix3fv(n.u_clip,!1,O(g,S,I,P,m)),e.enable(e.BLEND),e.blendFunc(e.SRC_ALPHA,e.ONE_MINUS_SRC_ALPHA),e.drawArraysInstanced(e.TRIANGLE_STRIP,0,4,f),e.disable(e.BLEND),e.bindVertexArray(null),e.bindBuffer(e.ARRAY_BUFFER,null))}},end(){a.end();let g=0,w=a.drain((E,S)=>{g++;let I=s.get(E);if(I===void 0)return;s.delete(E);let P=o.get(I);if(!P)return;let x=P.slots.claim(E,S.width,S.height);x&&(e.bindTexture(e.TEXTURE_2D_ARRAY,P.texture),e.texSubImage3D(e.TEXTURE_2D_ARRAY,0,0,0,x.layer,S.width,S.height,1,e.RGBA,e.UNSIGNED_BYTE,S),e.bindTexture(e.TEXTURE_2D_ARRAY,null))});return g>0||w>0},abandon(g){a.abandon(xt(g))},fetched:()=>a.fetched(),dispose(){a.stop(),e.deleteProgram(r),e.deleteVertexArray(u),e.deleteBuffer(p),e.deleteBuffer(c);for(let g of o.values())e.deleteTexture(g.texture);o.clear()}}}function Ct(e){let t="",r="",n=null,i="",o=null,s=null,a=0,u=0,p={map:It(),alpha:0},c={map:It(),alpha:0},l=[];function d(){if(!s)return 1;let f=(u-a)/250;return f>=1?1:f<=0?0:f}return{update(f,m){u=m,i=f.activeLayer,f.activeLayer!==r&&(r=f.activeLayer,t=""),n=Lt(f,t)??Lt(f,f.activeLayer);let y=n?.map??null;Mt(y)!==Mt(o)&&(s=o,o=y,a=s?m:m-250),s&&d()>=1&&(s=null)},choose(f){e&&(t=f===i?"":f)},viewed:()=>n,draws(){l.length=0;let f=d();return s&&f<1&&(p.map=s,p.alpha=1,l.push(p)),o&&(c.map=o,c.alpha=s?f:1,l.push(c)),l},fading:()=>s!==null,following:()=>n===null||n.id===i}}function Lt(e,t){if(t==="")return null;for(let r of e.layers)if(r.id===t)return r;return null}function Mt(e){return e?`${e.assetId}:${e.gen}`:""}function It(){return{assetId:"",gen:"",width:0,height:0,tileSize:0,maxZoom:0}}function kt(e){let{mount:t,canvas:r,render:n}=e,i=new Float64Array(600),o=0,s=0,a=0,u=!1;function p(){if(a=0,u)return;let y=performance.now(),h=n(),b=performance.now()-y;i[s]=b,s=(s+1)%600,o<600&&o++,h&&c()}function c(){a===0&&!u&&(a=requestAnimationFrame(p))}let l=new ResizeObserver(()=>{let y=window.devicePixelRatio||1,h=t.getBoundingClientRect(),b=Math.max(1,Math.round(h.width*y)),v=Math.max(1,Math.round(h.height*y));(r.width!==b||r.height!==v)&&(r.width=b,r.height=v),e.resized?.(),c()});l.observe(t);let d=null;function f(){d?.removeEventListener("change",m),d=window.matchMedia(`(resolution: ${window.devicePixelRatio||1}dppx)`),d.addEventListener("change",m)}function m(){f();let y=window.devicePixelRatio||1,h=t.getBoundingClientRect();r.width=Math.max(1,Math.round(h.width*y)),r.height=Math.max(1,Math.round(h.height*y)),e.resized?.(),c()}return f(),{invalidate:c,timings(){if(o===0)return{average:0,p95:0,samples:0};let y=Array.from(i.subarray(0,o)).sort((b,v)=>b-v),h=0;for(let b of y)h+=b;return{average:h/o,p95:y[Math.min(o-1,Math.floor(o*.95))]??0,samples:o}},resetTimings(){o=0,s=0},stop(){u=!0,a!==0&&(cancelAnimationFrame(a),a=0),l.disconnect(),d?.removeEventListener("change",m)}}}function Dt(e,t,r){let n=0;for(let i of e){if(i.layerId!==t)continue;let o=r[n]??(r[n]=Wr());o.id=i.id,o.kind=i.kind,o.name=i.name,o.image=i.image,o.x=i.x,o.y=i.y,o.z=i.z,o.size=i.size,o.footprintW=i.footprintW,o.footprintH=i.footprintH,o.hidden=!i.visible,o.dead=i.hp!==null&&i.hp<=0,n++}return r.length=n,r}function Ft(e,t,r){return e+(3+t*5)*r}function Wr(){return{id:"",kind:"monster",name:"",image:"",x:0,y:0,z:0,size:"medium",footprintW:0,footprintH:0,hidden:!1,dead:!1}}var zt=["player","monster","npc","object"],Bt=["tiny","small","medium","large","huge","gargantuan"];function Nt(e,t,r,n,i){let o=Math.max(1,r),s=t.map(c=>c.image).filter(c=>c!==""),a=[],u=24301,p=()=>(u=u*1664525+1013904223>>>0,u/4294967296);for(let c=0;c<e;c++){let l=zt[Math.floor(p()*zt.length)]??"monster",d=l==="object";a.push({id:`stress-${c.toString().padStart(4,"0")}`,kind:l,name:`Stress ${c}`,image:s.length>0?s[Math.floor(p()*s.length)]:"",x:n+Math.round((p()-.5)*30*o),y:i+Math.round((p()-.5)*30*o),z:c,size:d?"medium":Bt[Math.floor(p()*Bt.length)]??"medium",footprintW:d?1+Math.floor(p()*4):0,footprintH:d?1+Math.floor(p()*4):0,hidden:p()<.15,dead:p()<.1})}return a}var Ht=Math.log(1.25),Yr=.0015,Xr=.01,Or=16,qr=400;function Vr(){return{panX:0,panY:0,zoom:1,zoomX:0,zoomY:0}}function Gt(e,t,r){return e.panX!==0||e.panY!==0||e.zoom!==1?((e.panX!==0||e.panY!==0)&&it(t,e.panX,e.panY),e.zoom!==1&&Z(t,r,e.zoomX,e.zoomY,e.zoom),e.panX=0,e.panY=0,e.zoom=1,!0):!1}function $r(e,t,r){let n=e;t===1?n*=Or:t===2&&(n*=qr);let i=-n*(r?Xr:Yr);return Math.exp(Math.min(Math.max(i,-Ht),Ht))}var Kr=new Set([0,1]);function Wt(e,t){let r=Vr(),n=new Map;function i(p){let c=e.getBoundingClientRect();return{x:p.clientX-c.left,y:p.clientY-c.top}}function o(p){Kr.has(p.button)&&(p.preventDefault(),e.setPointerCapture(p.pointerId),n.set(p.pointerId,i(p)))}function s(p){let c=n.get(p.pointerId);if(!c)return;let l=i(p);if(n.size===1){r.panX+=l.x-c.x,r.panY+=l.y-c.y,c.x=l.x,c.y=l.y,t();return}let[d,f]=jr(n);if(!d||!f)return;let m=Ut(d,f),y=(d.x+f.x)/2,h=(d.y+f.y)/2;c.x=l.x,c.y=l.y;let b=Ut(d,f),v=(d.x+f.x)/2,R=(d.y+f.y)/2;r.panX+=v-y,r.panY+=R-h,m>1&&b>1&&(r.zoom*=b/m,r.zoomX=v,r.zoomY=R),t()}function a(p){n.delete(p.pointerId)&&e.hasPointerCapture(p.pointerId)&&e.releasePointerCapture(p.pointerId)}function u(p){p.preventDefault();let c=i(p);r.zoom*=$r(p.deltaY,p.deltaMode,p.ctrlKey),r.zoomX=c.x,r.zoomY=c.y,t()}return e.addEventListener("pointerdown",o),e.addEventListener("pointermove",s),e.addEventListener("pointerup",a),e.addEventListener("pointercancel",a),e.addEventListener("wheel",u,{passive:!1}),{pending:r,dragging:()=>n.size>0,stop(){e.removeEventListener("pointerdown",o),e.removeEventListener("pointermove",s),e.removeEventListener("pointerup",a),e.removeEventListener("pointercancel",a),e.removeEventListener("wheel",u),n.clear()}}}function jr(e){let t=e.values();return[t.next().value,t.next().value]}function Ut(e,t){return Math.hypot(e.x-t.x,e.y-t.y)}var Yt=1.5,Xt=1e4;function Ot(e,t){let r=e.querySelector("[data-tabletop-canvas]");if(!(r instanceof HTMLCanvasElement))return null;let n=r,i=st(n);if(!i)return e.querySelector("[data-tabletop-unsupported]")?.removeAttribute("hidden"),n.hidden=!0,null;let o=i,s=ut(o),a=Pt(o,()=>L.invalidate()),u=gt(o,()=>L.invalidate()),p=At(o),c=St(o),l=lt(o),d=Ie(o,l),f=Ie(o,l),m=Ct(e.dataset.role==="gm"),y=tt(),h={width:1,height:1},b=window.devicePixelRatio||1,v={r:0,g:0,b:0},R="",M=0,T=0,g=null,w=null,E="",S=!0,I=[],P=[],x=!0,C=0,H=-1,D=Wt(n,()=>L.invalidate()),L=kt({mount:e,canvas:n,render:re,resized(){b=window.devicePixelRatio||1;let _=e.getBoundingClientRect();h.width=Math.max(1,_.width),h.height=Math.max(1,_.height)}});j(),window.addEventListener("theme:change",j),window.addEventListener("room:view",ne),L.invalidate();function re(){let _=performance.now();m.update(t.table,_);let A=m.viewed()?.id??"",B=m.following();(A!==E||B!==S)&&(E=A,S=B,w?.());let W=m.draws(),k=m.viewed()?.map??null;V(k);let N=me(_,k);!N&&Gt(D.pending,y,h)&&k&&ye(y,h,k.width,k.height),o.viewport(0,0,n.width,n.height),o.clearColor(v.r,v.g,v.b,1),o.clear(o.COLOR_BUFFER_BIT),a.begin();for(let fe of W)a.draw(y,fe.map,fe.alpha,n.width,n.height,b);let z=a.end();s.draw(y,t.table.grid,n.width,n.height,b);let Ee=ue(A);return D.dragging()||m.fading()||z||Ee||N}function ue(_){let A=t.table.grid,B=Math.max(1,A.cellSize),W=x||B!==C||u.epoch()!==H;u.begin(W);let k=1/Math.max(y.zoom*b,1e-4),N=1/Math.max(y.zoom,1e-4);d.begin(N),f.begin(N),W&&(Dt(t.pawns,_,I),p.build(P.length>0?I.concat(P):I,A,u),x=!1,C=B,H=u.epoch()),d.draw(y,n.width,n.height,b),p.draw(y,n.width,n.height,b),c.begin();for(let z of t.pawns){if(z.layerId!==_||z.kind==="object"||z.conditions.length===0)continue;let[Ee]=Be(z,B),fe=Math.min(z.conditions.length,16);for(let pe=0;pe<fe;pe++){let nr=Fe[z.conditions[pe].color]??Fe.white,Ye=Ft(Ee,pe,k);c.add(z.x,z.y,Ye,Ye,nr,1,2,Tt)}}return c.draw(y,n.width,n.height,b),f.draw(y,n.width,n.height,b),u.end()}function V(_){let A=_?`${_.assetId}:${_.gen}`:"";A!==R&&(R=A,_&&((M!==_.width||T!==_.height)&&Le(y,h,_.width,_.height),M=_.width,T=_.height))}function me(_,A){if(!g)return!1;let B=_-g.from;if(B>=Xt||!A){let k=L.timings(),N=g.report,z=g.tiles;return g=null,N({average:k.average,p95:k.p95,frames:k.samples,tiles:a.fetched()-z}),!1}let W=B/Xt;if(W<.75){let k=Math.floor(W/.25),N=W%.25/.25;y.zoom=[K(A),1,2][k]??1,y.x=A.width*N,y.y=A.height/2}else{let k=(W-.75)/.25,N=1-Math.abs(k*2-1);y.x=A.width/2,y.y=A.height/2,y.zoom=K(A)*Math.pow(4/K(A),N)}return ye(y,h,A.width,A.height),!0}function K(_){return Math.min(h.width/_.width,h.height/_.height)*.9}function ne(_){let A=m.viewed()?.map??null;switch(_.detail?.action){case"zoom-in":Z(y,h,h.width/2,h.height/2,Yt);break;case"zoom-out":Z(y,h,h.width/2,h.height/2,1/Yt);break;case"zoom-1":Pe(y,h,1);break;case"zoom-2":Pe(y,h,2);break;case"fit":A&&Le(y,h,A.width,A.height);break;default:return}A&&ye(y,h,A.width,A.height),L.invalidate()}function j(){let _=document.createElement("canvas").getContext("2d",{willReadFrequently:!0});if(!_)return;_.fillStyle="#000000",_.fillStyle=window.getComputedStyle(e).backgroundColor,_.fillRect(0,0,1,1);let A=_.getImageData(0,0,1,1).data;v.r=A[0]/255,v.g=A[1]/255,v.b=A[2]/255,L.invalidate()}return{invalidate:L.invalidate,view:m,onSettled(_){w=_},benchmark(_){g||(L.resetTimings(),g={from:performance.now(),report:_,tiles:a.fetched()},L.invalidate())},pawnsChanged(){x=!0},stress(_){let A=m.viewed()?.map??null,B=Math.max(1,t.table.grid.cellSize);return P=_>0?Nt(_,I,B,A?A.width/2:0,A?A.height/2:0):[],x=!0,L.invalidate(),P.length},stop(){window.removeEventListener("theme:change",j),window.removeEventListener("room:view",ne),D.stop(),L.stop(),a.dispose(),s.dispose(),p.dispose(),c.dispose(),d.dispose(),f.dispose(),u.dispose(),l?.dispose()}}}var qt="/fragment/",de=null,ge=null,We="",Y=new Map,Vt=20,$t=0;function er(e,t){de=e,We=t;let r=document.querySelector("[data-window-template]");if(ge=r instanceof HTMLTemplateElement?r:null,!ge)return;document.addEventListener("click",i=>{if(!(i.target instanceof Element))return;let o=i.target.closest("[data-window]");if(!(o instanceof HTMLElement))return;let{window:s,windowUrl:a,windowTitle:u}=o.dataset;if(!s||!a||!u){console.error("a window trigger is missing an id, a url or a title",o.dataset);return}Kt({id:s,url:a,title:u,width:Qt(o.dataset.windowWidth),height:Qt(o.dataset.windowHeight)})});let n;window.addEventListener("resize",()=>{window.clearTimeout(n),n=window.setTimeout(()=>{for(let i of Y.values())i.reclamp()},150)}),window.addEventListener("window:close",i=>{let o=i.detail?.id;o&&Y.get(o)?.close()}),window.addEventListener("window:retitle",i=>{let o=i.detail;o?.id&&o.title&&Y.get(o.id)?.retitle(o.title)});for(let i of en())Kt(i)}function Kt(e){if(!e.url.startsWith(qt)){console.error(`a window needs a ${qt} url, refusing:`,e.url);return}if(!de||!ge)return;let t=Y.get(e.id);if(t){t.reveal();return}try{Y.set(e.id,new Ue(e,de,ge))}catch(r){console.error(`the ${e.id} window could not be opened`,r),Y.delete(e.id)}Ge()}var Ue=class{id;url;title;el;bar;content;panes;x=0;y=0;w=280;h=320;minimized=!1;restoreTo=null;request=null;constructor(t,r,n){this.id=t.id,this.title=t.title,this.url=t.url;let i=n.content.firstElementChild?.cloneNode(!0);if(!(i instanceof HTMLElement))throw new Error("the window template has no element in it");this.el=i,this.bar=ee(i,"[data-window-bar]"),this.content=ee(i,'[data-window-state="content"]'),this.panes=new Map([["loading",ee(i,'[data-window-state="loading"]')],["error",ee(i,'[data-window-state="error"]')],["content",this.content]]);let o=ee(i,"[data-window-title]");o.textContent=t.title,$t+=1,o.id=`window-title-${$t}`,i.setAttribute("aria-labelledby",o.id);let s=Qr(t.id);this.w=s?.w??t.width??280,this.h=s?.h??t.height??320,this.x=s?.x??16,this.y=s?.y??16,this.wire(),r.append(i),this.reclamp(),this.raise(),this.load()}wire(){this.el.addEventListener("pointerdown",()=>this.raise()),this.bar.addEventListener("pointerdown",t=>this.startDrag(t));for(let t of this.el.querySelectorAll("[data-window-resize]")){if(!(t instanceof HTMLElement))continue;let r=t.dataset.windowResize??"both";t.addEventListener("pointerdown",n=>this.startResize(n,r))}this.el.addEventListener("click",t=>{if(!(t.target instanceof Element))return;let r=t.target.closest("[data-window-action]");if(!(!(r instanceof HTMLElement)||this.content.contains(r)))switch(r.dataset.windowAction){case"minimize":this.collapse(!this.minimized);break;case"maximize":this.toggleMaximize();break;case"close":this.close();break;case"retry":this.load();break}}),this.el.addEventListener("htmx:before:request",t=>{t.target===this.el&&(this.request=t.detail.ctx)}),this.el.addEventListener("htmx:before:swap",t=>{t.target===this.el&&t.detail.ctx!==this.request&&t.preventDefault()}),this.el.addEventListener("htmx:finally:request",t=>{let r=t.detail;if(t.target!==this.el||r.ctx!==this.request)return;let n=r.ctx.response?.status;this.show(n!==void 0&&n<400?"content":"error")})}load(){if(this.show("loading"),this.content.replaceChildren(),this.request=null,typeof htmx>"u"){this.show("error");return}htmx.ajax("GET",this.url,{target:this.content,source:this.el})}show(t){for(let[r,n]of this.panes)n.hidden=r!==t}reveal(){this.collapse(!1),this.raise()}raise(){Vt+=1,this.el.style.zIndex=String(Vt)}paint(){this.el.style.transform=`translate(${this.x}px, ${this.y}px)`,this.el.style.width=`${this.w}px`,this.el.style.height=`${this.height()}px`}height(){return this.minimized?this.bar.offsetHeight:this.h}moveTo(t,r){let n=le(),i=this.height();this.x=q(Zt(t,this.w,this.lines("x")),0,Math.max(0,n.w-this.w)),this.y=q(Zt(r,i,this.lines("y")),0,Math.max(0,n.h-i)),jt(this)}resizeTo(t,r){let n=le();if(t!==null){let i=Jt(this.x+t,this.lines("x"));this.w=q(i-this.x,200,Math.max(200,n.w-this.x))}if(r!==null){let i=Jt(this.y+r,this.lines("y"));this.h=q(i-this.y,120,Math.max(120,n.h-this.y))}jt(this)}lines(t){let r=le(),n=[0,t==="x"?r.w:r.h];for(let i of Y.values())i!==this&&(t==="x"?n.push(i.x,i.x+i.w):n.push(i.y,i.y+i.height()));return n}reclamp(){let t=le();if(this.restoreTo){this.x=0,this.y=0,this.w=t.w,this.h=t.h,this.commit();return}this.w=q(this.w,200,Math.max(200,t.w)),this.h=q(this.h,120,Math.max(120,t.h)),this.x=q(this.x,0,Math.max(0,t.w-this.w)),this.y=q(this.y,0,Math.max(0,t.h-this.height())),this.commit()}collapse(t){this.minimized=t,this.el.querySelector("[data-window-body]")?.toggleAttribute("hidden",t),this.reclamp()}toggleMaximize(){let t=le();if(this.restoreTo){let r=this.restoreTo;this.restoreTo=null,this.x=r.x,this.y=r.y,this.w=r.w,this.h=r.h}else this.restoreTo={x:this.x,y:this.y,w:this.w,h:this.h},this.x=0,this.y=0,this.w=t.w,this.h=t.h;this.icon("maximize",this.restoreTo!==null),this.icon("restore",this.restoreTo===null),this.reclamp()}icon(t,r){let n=this.el.querySelector(`[data-window-icon="${t}"]`);n instanceof HTMLElement&&(n.hidden=r)}close(){this.save(),this.el.remove(),Y.delete(this.id),Ge()}save(){tr(`window:${this.id}`,this.restoreTo??{x:this.x,y:this.y,w:this.w,h:this.h})}retitle(t){t===""||t===this.title||(this.title=t,ee(this.el,"[data-window-title]").textContent=t,Ge())}spec(){return{id:this.id,title:this.title,url:this.url}}commit(){this.paint()}startDrag(t){if(t.target instanceof Element&&t.target.closest("button")||this.restoreTo)return;let r=t.clientX-this.x,n=t.clientY-this.y;this.drag(this.bar,t,i=>this.moveTo(i.clientX-r,i.clientY-n))}startResize(t,r){if(this.restoreTo||this.minimized)return;t.preventDefault();let n=t.clientX-this.w,i=t.clientY-this.h;this.drag(t.currentTarget,t,o=>{this.resizeTo(r==="y"?null:o.clientX-n,r==="x"?null:o.clientY-i)})}drag(t,r,n){this.raise(),t.setPointerCapture(r.pointerId);let i=s=>n(s),o=()=>{t.removeEventListener("pointermove",i),t.removeEventListener("pointerup",o),t.removeEventListener("pointercancel",o),this.save()};t.addEventListener("pointermove",i),t.addEventListener("pointerup",o),t.addEventListener("pointercancel",o)}},Ne=new Set,He=0;function jt(e){Ne.add(e),He===0&&(He=requestAnimationFrame(()=>{He=0;for(let t of Ne)t.commit();Ne.clear()}))}function le(){return{w:de?.clientWidth??0,h:de?.clientHeight??0}}function q(e,t,r){return Math.min(Math.max(e,t),r)}function Zt(e,t,r){let n=e,i=10;for(let o of r){let s=Math.abs(e-o);s<i&&(i=s,n=o);let a=Math.abs(e+t-o);a<i&&(i=a,n=o-t)}return n}function Jt(e,t){let r=e,n=10;for(let i of t){let o=Math.abs(e-i);o<n&&(n=o,r=i)}return r}function ee(e,t){let r=e.querySelector(t);if(!(r instanceof HTMLElement))throw new Error(`the window template has no ${t}`);return r}function Qt(e){let t=Number.parseInt(e??"",10);return Number.isFinite(t)?t:void 0}function tr(e,t){try{localStorage.setItem(`tabletopper:${e}`,JSON.stringify(t))}catch{}}function rr(e){try{let t=localStorage.getItem(`tabletopper:${e}`);return t===null?null:JSON.parse(t)}catch{return null}}function Qr(e){let t=rr(`window:${e}`);return!t||typeof t.x!="number"||typeof t.y!="number"?null:{x:t.x,y:t.y,w:typeof t.w=="number"?t.w:280,h:typeof t.h=="number"?t.h:320}}function Ge(){let e=[];for(let t of Y.values())e.push(t.spec());tr(`windows:${We}`,e)}function en(){let e=rr(`windows:${We}`);return Array.isArray(e)?e.filter(t=>typeof t?.id=="string"&&typeof t?.title=="string"&&typeof t?.url=="string"):[]}var te=document.getElementById("tabletop");if(te){er(te,te.dataset.room??"");let e=$e(),t=Ot(te,e);if(t){let n=Je(te,e,t);n&&t.onSettled(n.refresh)}let r=te.dataset.socket??"";r!==""&&rn(r,e,t)}function tn(e){return e==="snapshot"||e==="table.updated"||e.startsWith("pawn.")&&e!=="pawn.dragging"}function rn(e,t,r){let n=null,i=new be(e,{event(s){Ve(t,s),qe(s),n?.event(s),r?.invalidate(),tn(s.type)&&r?.pawnsChanged(),s.type==="player.kicked"&&Ze(s.reason)},status(s,a){n?.status(s,a)}}),o=document.querySelector("[data-room-debug]");o instanceof HTMLElement&&(n=je(o,i,t,r)),i.start()}
//# sourceMappingURL=room.js.map
