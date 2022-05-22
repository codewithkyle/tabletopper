import r from"./jsql.js";import a from"./supercomponent.js";import{html as t,render as l}from"./lit-html.js";import i from"./env.js";import s from"./player-token-modal.js";class n extends a{constructor(){super();this.openModal=e=>{const o=new s(this.imageUploadCallback.bind(this));document.body.appendChild(o)};this.model={selectedImageId:null}}async connected(){await i.css(["player-token-picker","player-token-picker-modal"]),this.render()}imageUploadCallback(e){e!==null&&this.set({selectedImageId:e})}async getValue(){return(await r.query("SELECT * FROM images WHERE uid = $uid",{uid:this.model.selectedImageId}))?.[0]??null??null}async renderImage(){const e=(await r.query("SELECT * FROM images WHERE uid = $uid",{uid:this.model.selectedImageId}))[0];return t`
            <button @click=${this.openModal} class="picker" sfx="button" tooltip="Change token">
                <img src="${e.data}" alt="${e.name}" draggable="false">
            </button>
        `}renderEmpty(){return t`
            <button @click=${this.openModal} class="picker" sfx="button" tooltip="Select token">
                <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-photo" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                    <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                    <line x1="15" y1="8" x2="15.01" y2="8"></line>
                    <rect x="4" y="4" width="16" height="16" rx="3"></rect>
                    <path d="M4 15l4 -4a3 5 0 0 1 3 0l5 5"></path>
                    <path d="M14 14l1 -1a3 5 0 0 1 3 0l2 2"></path>
                </svg>
            </button>
        `}async render(){const e=t`
            ${this.model.selectedImageId?await this.renderImage():this.renderEmpty()}
        `;l(e,this)}}i.bind("player-token-picker",n);export{n as default};
