import d from"./supercomponent.js";import{html as o,render as m}from"./lit-html.js";import r from"./env.js";import h from"./resize-handle.js";class l extends d{constructor(t){super();this.move=t=>{if(this.moving){let e,i;t instanceof TouchEvent?(e=t.touches[0].clientX,i=t.touches[0].clientY):(e=t.clientX,i=t.clientY);const n=this.getBoundingClientRect();let s=n.x-e-this.localX,a=n.y-i-this.localY;this.x-=s,this.y-=a,this.style.transform=`translate(${this.x}px, ${this.y}px)`}};this.stopMove=t=>{this.moving&&this.save(),this.moving=!1};this.startMove=t=>{if(t.preventDefault(),t.stopImmediatePropagation(),this.model.size==="maximized")return;this.moving=!0;let e,i;t instanceof TouchEvent?(e=t.touches[0].clientX,i=t.touches[0].clientY):(e=t.clientX,i=t.clientY);const n=this.getBoundingClientRect();this.localX=n.x-e,this.localY=n.y-i,this.focus()};this.handleMinimize=t=>{this.minimize()};this.handleMaximize=t=>{switch(this.model.size){case"normal":this.maximize();break;default:this.windowed();break}};this.handleClose=t=>{this.close()};this.noop=t=>{t.preventDefault(),t.stopImmediatePropagation()};this.handle=t?.handle??t.name.toLowerCase().trim().replace(/\s+/g,"-"),this.minWidth=t?.minWidth??411,this.minHeight=t?.minHeight??231;const e=localStorage.getItem(`${this.handle}-x`),i=localStorage.getItem(`${this.handle}-y`);this.x=e?parseInt(e):0,this.y=i?parseInt(i):28,this.style.transform=`translate(${this.x}px, ${this.y}px)`,e==null&&localStorage.setItem(`${this.handle}-x`,this.x.toFixed(0).toString()),i==null&&localStorage.setItem(`${this.handle}-y`,this.y.toFixed(0).toString());const n=localStorage.getItem(`${this.handle}-w`),s=localStorage.getItem(`${this.handle}-h`);n!=null&&s!=null?(this.w=parseInt(n),this.h=parseInt(s)):(this.w=t?.width??this.minWidth,this.h=t?.height??this.minHeight),n==null&&localStorage.setItem(`${this.handle}-w`,this.w.toFixed(0).toString()),s==null&&localStorage.setItem(`${this.handle}-h`,this.h.toFixed(0).toString()),this.view=t.view,this.name=t.name,this.moving=!1,this.model={size:"normal"}}async connected(){await r.css(["window"]),window.addEventListener("mouseup",this.stopMove,{capture:!0,passive:!0}),window.addEventListener("mousemove",this.move,{capture:!0,passive:!0}),this.render(),this.focus()}focus(){document.body.querySelectorAll("window-component").forEach(t=>{t.blur()}),this.style.zIndex="1001"}blur(){this.style.zIndex="1000"}maximize(){this.moving=!1,this.x=0,this.y=0,this.w=window.innerWidth,this.h=window.innerHeight,this.style.transform=`translate(${this.x}px, ${this.y}px)`,this.set({size:"maximized"}),this.focus()}minimize(){this.moving=!1,this.h=28,this.w=parseInt(localStorage.getItem(`${this.handle}-w`)),this.set({size:"minimized"})}windowed(){this.moving=!1,this.x=parseInt(localStorage.getItem(`${this.handle}-x`)),this.y=parseInt(localStorage.getItem(`${this.handle}-y`)),this.w=parseInt(localStorage.getItem(`${this.handle}-w`)),this.h=parseInt(localStorage.getItem(`${this.handle}-h`)),this.style.transform=`translate(${this.x}px, ${this.y}px)`,this.set({size:"normal"}),this.focus()}close(){this.remove()}save(){const t=this.getBoundingClientRect();this.x=t.x,this.y=t.y,this.w=t.width,this.w<this.minWidth&&(this.w=this.minWidth),this.h=t.height,this.h<this.minHeight&&(this.h=this.minHeight),localStorage.setItem(`${this.handle}-w`,this.w.toFixed(0).toString()),localStorage.setItem(`${this.handle}-h`,this.h.toFixed(0).toString()),this.model.size!=="maximized"&&(localStorage.setItem(`${this.handle}-x`,this.x.toFixed(0).toString()),localStorage.setItem(`${this.handle}-y`,this.y.toFixed(0).toString()))}renderMaximizeIcon(){switch(this.model.size){case"normal":return o`
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-square" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <rect x="4" y="4" width="16" height="16" rx="2"></rect>
                    </svg>
                `;default:return o`
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-copy" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <rect x="8" y="8" width="12" height="12" rx="2"></rect>
                        <path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2"></path>
                    </svg>
                `}}renderContent(){let t;return this.model.size!=="minimized"?t=o`
                <div class="container">
                    ${this.view}
                </div>
            `:t="",t}render(){this.style.width=`${this.w}px`,this.style.height=`${this.h}px`,this.setAttribute("window",this.name.toLowerCase().trim().replace(/\s+/g,"-")),this.setAttribute("size",this.model.size);const t=o`
            <div class="header" flex="row nowrap items-center justify-between" @mousedown=${this.startMove}>
                <h3 class="font-sm px-0.5">${this.name}</h3>
                <div class="h-full" flex="row nowrap items-center">
                    <button @click=${this.handleMinimize} @mousedown=${this.noop} sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-minus" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <line x1="5" y1="12" x2="19" y2="12"></line>
                        </svg>
                    </button>
                    <button @click=${this.handleMaximize} @mousedown=${this.noop} sfx="button">
                        ${this.renderMaximizeIcon()}
                    </button>
                    <button @click=${this.handleClose} @mousedown=${this.noop} sfx="button">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <line x1="18" y1="6" x2="6" y2="18"></line>
                            <line x1="6" y1="6" x2="18" y2="18"></line>
                        </svg>
                    </button>
                </div>
            </div>
            ${this.renderContent()}
            ${new h(this,"x",this.minWidth,this.minHeight)}
            ${new h(this,"y",this.minWidth,this.minHeight)}
            ${new h(this,"both",this.minWidth,this.minHeight)}
        `;m(t,this)}}r.bind("window-component",l);export{l as default};
