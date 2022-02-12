import t from"./supercomponent.js";import{html as i,render as l}from"./lit-html.js";import o from"./env.js";class s extends t{constructor(e="small"){super();this.close=e=>{this.remove()};this.model={view:"",heading:null,message:null,size:e}}connected(){this.render()}setHeading(e){this.isConnected?this.set({heading:e}):this.model.heading=e}setMessage(e){this.isConnected?this.set({message:e}):this.model.message=e}setView(e){this.isConnected?this.set({view:e}):this.model.view=e}render(){const e=i`
            <div @click=${this.close} class="backdrop"></div>
			<div class="modal" size="${this.model.size}">
				${this.model.heading?.length?i`<h1>${this.model.heading}</h1>`:""}
				${this.model.message?.length?i`<p>${this.model.message}</p>`:""}
				<button class="close bttn" kind="text" color="grey" icon="center" shape="round" aria-label="close modal" @click=${this.close}>
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <line x1="18" y1="6" x2="6" y2="18"></line>
                        <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
				</button>
                ${this.model.view}
			</div>
        `;l(e,this)}}o.bind("modal-component",s);export{s as default};
