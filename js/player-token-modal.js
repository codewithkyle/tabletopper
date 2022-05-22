import i from"./jsql.js";import c from"./supercomponent.js";import{html as l,render as d}from"./lit-html.js";import p from"./env.js";import{UUID as m}from"./uuid.js";import{Base64EncodeFile as u}from"./file.js";class r extends c{constructor(e){super();this.close=e=>{this.model.callback(null),this.remove()};this.handleInput=async e=>{const t=e.currentTarget.files;if(t.length){const o=t[0],s=await u(o),a=m();await i.query("INSERT INTO images VALUES ($img)",{img:{uid:a,name:o.name,data:s,type:"player-token"}}),this.model.callback(a),this.remove()}};this.selectImage=e=>{const n=e.currentTarget;this.model.callback(n.dataset.uid),this.remove()};this.model={callback:e},this.render()}async render(){const e=await i.query("SELECT * FROM images WHERE type = 'player-token'"),n=l`
           <div class="backdrop" @click=${this.close}></div>
           <div class="modal">
               <button class="close" @click=${this.close}>
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"/>
                        <line x1="18" y1="6" x2="6" y2="18" />
                        <line x1="6" y1="6" x2="18" y2="18" />
                    </svg>
               </button>
               <h2 class="block font-lg pb-0.5 mb-1 font-medium font-grey-900 line-normal border-b-1 border-b-solid border-b-grey-200">Your Tokens</h2>
               <div class="content">
                   <div class="upload-image-button" sfx="button">
                       <label for="token-upload">
                            <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-upload" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"></path>
                                <polyline points="7 9 12 4 17 9"></polyline>
                                <line x1="12" y1="4" x2="12" y2="16"></line>
                            </svg>
                            <span>Upload</span>
                       </label>
                       <input @change=${this.handleInput} type="file" accept="image/png, image/jpg, image/jpeg" id="token-upload" name="token">
                    </div>
                   ${e.map(t=>l`
                            <button sfx="button" class="token-button" data-uid="${t.uid}" @click=${this.selectImage}>
                                <img src="${t.data}" alt="${t.name}" draggalbe="false">
                            </button>
                        `)}
               </div>
           </div>
        `;d(n,this)}}p.bind("player-token-picker-modal",r);export{r as default};
