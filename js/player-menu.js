import c from"./jsql.js";import{publish as d,subscribe as p}from"./pubsub.js";import m from"./supercomponent.js";import{html as r,render as u}from"./lit-html.js";import h from"./input.js";import i from"./env.js";import b from"./modal.js";import o from"./control-center.js";import{send as v}from"./ws.js";class s extends m{constructor(){super();this.locatePawn=e=>{const t=e.currentTarget;d("tabletop",{type:"locate:pawn",data:t.dataset.uid})};this.kickPlayer=e=>{const t=e.currentTarget;v("room:player:ban",t.dataset.uid)};this.handleRenameSubmit=e=>{const t=e.currentTarget,l=t.querySelector(".js-input").getValue(),a=o.set("players",t.dataset.playerUid,"name",l);o.perform(a,!0),o.dispatch(a),t.closest("modal-component").remove()};this.renamePlayer=e=>{const t=e.currentTarget,n=new b("medium");n.setHeading("Rename Player"),n.setView(r`
            <form @submit=${this.handleRenameSubmit.bind(this)} data-player-uid="${t.dataset.uid}">
                <div class="w-full px-1 pb-1 pt-0.5">
                    ${new h({name:"name",value:t.dataset.name})}
                </div>
                <div class="actions">
                    <button class="bttn mr-1" kind="solid" color="white" type="button" @click=${n.close.bind(n)}>cancel</button>
                    <button class="bttn" kind="solid" color="success" type="submit">update</button>
                </div>
            </form>
        `),document.body.append(n)};p("sync",this.syncInbox.bind(this))}syncInbox(e){e.table==="players"&&this.render()}async connected(){await i.css(["player-menu","button"]),this.render()}renderPlayer(e){return r`
            <div flex="row nowrap items-center justify-between" class="w-full player border-1 border-solid border-grey-300 radius-0.25 pl-0.75">
                <span title="${e.name}" class="cursor-default font-sm font-medium font-grey-700">${e.name}</span>
                <div class="h-full" flex="row nowrap items-center">
                    <button data-uid="${e.uid}" @click=${this.locatePawn} sfx="button" tooltip="Go to pawn" class="bttn border-r-1 border-l-1 border-l-solid border-l-grey-300 border-r-solid border-r-grey-300" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-current-location" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <circle cx="12" cy="12" r="3"></circle>
                            <circle cx="12" cy="12" r="8"></circle>
                            <line x1="12" y1="2" x2="12" y2="4"></line>
                            <line x1="12" y1="20" x2="12" y2="22"></line>
                            <line x1="20" y1="12" x2="22" y2="12"></line>
                            <line x1="2" y1="12" x2="4" y2="12"></line>
                        </svg>
                    </button>
                    <button data-name="${e.name}" data-uid="${e.uid}" @click=${this.renamePlayer} sfx="button" tooltip="Rename player" class="bttn border-r-1 border-r-solid border-r-grey-300" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-forms" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M12 3a3 3 0 0 0 -3 3v12a3 3 0 0 0 3 3"></path>
                            <path d="M6 3a3 3 0 0 1 3 3v12a3 3 0 0 1 -3 3"></path>
                            <path d="M13 7h7a1 1 0 0 1 1 1v8a1 1 0 0 1 -1 1h-7"></path>
                            <path d="M5 7h-1a1 1 0 0 0 -1 1v8a1 1 0 0 0 1 1h1"></path>
                            <path d="M17 12h.01"></path>
                            <path d="M13 12h.01"></path>
                        </svg> 
                    </button>
                    <button data-uid="${e.uid}" @click=${this.kickPlayer} sfx="button" tooltip="Kick ${e.name}" class="bttn" kind="text" color="grey" icon="center">
                        <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-user-off" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                            <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                            <path d="M14.274 10.291a4 4 0 1 0 -5.554 -5.58m-.548 3.453a4.01 4.01 0 0 0 2.62 2.65"></path>
                            <path d="M6 21v-2a4 4 0 0 1 4 -4h4a4 4 0 0 1 1.147 .167m2.685 2.681a4 4 0 0 1 .168 1.152v2"></path>
                            <line x1="3" y1="3" x2="21" y2="21"></line>
                        </svg>
                    </button>
                </div>
            </div>
        `}async render(){const e=await c.query("SELECT * FROM players WHERE room = $room AND active = 1",{room:sessionStorage.getItem("room")}),t=r`
            <div class="w-full block p-0.5">
                ${e.map(this.renderPlayer.bind(this))}
            </div>
        `;u(t,this)}}i.bind("player-menu",s);export{s as default};
