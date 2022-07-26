import a from"./jsql.js";import p from"./supercomponent.js";import{UUID as h}from"./uuid.js";import{html as o,render as g}from"./lit-html.js";import l from"./button.js";import u from"./spinner.js";import n from"./env.js";import b from"./notifications.js";import d from"./control-center.js";import{send as v}from"./ws.js";import{Base64EncodeFile as f}from"./file.js";class c extends p{constructor(){super();this.clickClose=e=>{this.close()};this.handleInput=async e=>{const t=e.currentTarget.files;if(t.length){const s=t[0];if(s.size<1e7){this.trigger("LOAD");const m=await f(s),r=h();await a.query("INSERT INTO images VALUES ($img)",{img:{uid:r,name:s.name,data:m,type:"map"}}),this.set({selected:r})}else b.error("Upload Failed","Files must be 10MB or smaller.")}};this.selectImage=e=>{const t=e.currentTarget.dataset.uid;this.set({selected:t})};this.model={selected:null,images:[]},this.state="LOADING",this.stateMachine={LOADING:{READY:"IDLING",LOAD:" LOADING"},IDLING:{LOAD:"LOADING"}}}async connected(){await n.css(["tabletop-image-modal"]),this.render();const e=await a.query("SELECT * FROM images WHERE type = map");this.update({images:e}),this.trigger("READY")}close(){this.remove()}async load(){const e=(await a.query("SELECT loaded_maps FROM games WHERE room = $room",{room:sessionStorage.getItem("room")}))[0].loaded_maps;let i=!1;for(let t=0;t<e.length;t++)if(e[t]===this.model.selected){i=!0;break}if(!i){const t=(await a.query("SELECT * FROM images WHERE uid = $uid",{uid:this.model.selected}))[0],s=d.insert("images",t.uid,t);d.dispatch(s)}v("room:tabletop:map:load",this.model.selected),this.remove()}async renderImages(){return o`
            <div class="images">
                <div class="upload-image-button" sfx="button">
                    <label for="upload">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-upload" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"></path>
                            <polyline points="7 9 12 4 17 9"></polyline>
                            <line x1="12" y1="4" x2="12" y2="16"></line>
                        </svg>
                        <span>Upload</span>
                    </label>
                    <input @change=${this.handleInput} type="file" accept="image/png, image/jpg, image/jpeg" id="upload" name="token">
                </div>
                ${this.model.images.map(e=>o`
                        <button sfx="button" class="token-button ${this.model.selected===e.uid?"is-selected":""}" data-uid="${e.uid}" @click=${this.selectImage} aria-label="${e.name}" tooltip>
                            <img src="${e.data}" alt="${e.name}" draggalbe="false">
                            ${this.model.selected===e.uid?o`
                                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-circle-check" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                                    <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                                    <circle cx="12" cy="12" r="9"></circle>
                                    <path d="M9 12l2 2l4 -4"></path>
                                </svg>
                            `:""}
                        </button>
                    `)}
            </div>
        `}async render(){let e;switch(this.state){case"LOADING":e=o`
                    ${new u({size:32,color:"grey",css:"z-index: 5;"})}
                `;break;default:e=o`
                    <div class="modal">
                        <h2 class="font-grey-800 font-bold block px-1.5 pt-1.25 pb-1.25 border-b-solid border-b-1 border-b-grey-300">Tabletop Images</h2>
                        ${new l({callback:this.close.bind(this),kind:"text",color:"grey",icon:'<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>',class:"close",iconPosition:"center",shape:"round"})}
                        ${await this.renderImages()}
                        <div class="w-full border-t-solid border-t-1 border-t-grey-200 bg-grey-50 p-1" flex="row nowrap items-center justify-end">
                            ${new l({callback:this.close.bind(this),kind:"solid",color:"white",label:"cancel",class:"mr-1"})}
                            ${new l({kind:"solid",color:"success",callback:this.load.bind(this),label:"load image",disabled:!this.model.selected?.length})}
                        </div>
                    </div>
                `;break}const i=o`
            <div class="backdrop" @click=${this.clickClose}></div>
            ${e}
        `;g(i,this)}}n.bind("tabletop-image-modal",c);export{c as default};
