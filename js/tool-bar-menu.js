import{publish as o,subscribe as p}from"./pubsub.js";import l from"./supercomponent.js";import{html as n,render as u}from"./lit-html.js";import i from"./env.js";import b from"./notifications.js";import c from"./tabletop-image-modal.js";import a from"./window.js";import m from"./monster-manual.js";import d from"./player-menu.js";import f from"./spellbook.js";import{send as s}from"./ws.js";class r extends l{constructor(t){super();this.clearImage=t=>{s("room:tabletop:map:clear"),this.close()};this.clickExit=t=>{this.exit()};this.clickLoadImage=t=>{this.loadImage()};this.toggleFullscreen=t=>{window.innerHeight===screen.height?document.exitFullscreen():document.documentElement.requestFullscreen(),this.close()};this.zoomReset=t=>{o("tabletop",{type:"zoom",data:{zoom:1,x:null,y:null,delta:this.zoom<=1?-100:100}}),this.close()};this.zoom200=t=>{o("tabletop",{type:"zoom",data:{zoom:2,x:null,y:null,delta:100}}),this.close()};this.zoomOut=t=>{if(this.zoom===.1)return;let e=this.zoom-.1;e<.1&&(e=.1),o("tabletop",{type:"zoom",data:{zoom:e,x:null,y:null,delta:-100}})};this.zoomIn=t=>{if(this.zoom===2)return;let e=this.zoom+.1;e>2&&(e=2),o("tabletop",{type:"zoom",data:{zoom:e,x:null,y:null,delta:100}})};this.centerTabletop=t=>{o("tabletop",{type:"position:reset"}),this.close()};this.spawnPawns=async t=>{s("room:tabletop:spawn:players"),this.close()};this.copyRoomCode=async t=>{await navigator.clipboard.writeText(sessionStorage.getItem("room")),b.snackbar("Room code copied to clipboard."),this.close()};this.lockRoom=t=>{s("room:lock"),this.close()};this.unlockRoom=t=>{s("room:unlock"),this.close()};this.openPlayerMenu=t=>{const e=document.body.querySelector('window-component[window="players"]')||new a({name:"Players",view:new d});e.isConnected||document.body.append(e),this.close()};this.openMonsterManual=t=>{const e=document.body.querySelector('window-component[window="monster-manual"]')||new a({name:"Monster Manual",view:new m});e.isConnected||document.body.append(e),this.close()};this.openSpellbook=t=>{const e=document.body.querySelector('window-component[window="spellbook"]')||new a({name:"Spellbook",view:new f,minWidth:650,minHeight:480,width:900});e.isConnected||document.body.append(e),this.close()};this.clickBackdrop=t=>{this.close()};this.zoom=sessionStorage.getItem("zoom")!=null?parseFloat(sessionStorage.getItem("zoom")):1,this.model={menu:t},p("tabletop",this.tabletopInbox.bind(this))}tabletopInbox({type:t,data:e}){switch(t){case"zoom":this.zoom=e.zoom;break;default:break}}async connected(){i.css(["tool-bar-menu"]),this.render()}changeMenu(t){this.set({menu:t})}calcOffsetX(){return document.body.querySelector(`tool-bar button[data-menu="${this.model.menu}"]`).getBoundingClientRect().x}exit(){sessionStorage.removeItem("room"),sessionStorage.removeItem("lastSocketId"),s("room:quit"),this.close(),location.href=location.origin}close(){o("toolbar",{type:"menu:close"}),this.remove()}loadImage(){const t=new c;document.body.append(t),this.close()}renderFileMenu(){return sessionStorage.getItem("role")==="gm"?n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Save</span>
                        <span>Ctrl+S</span>
                    </button>
                    <button sfx="button">
                        <span>Save As</span>
                        <span>Ctrl+Shift+S</span>
                    </button>
                    <button sfx="button">
                        <span>Load</span>
                        <span>Ctrl+L</span>
                    </button>
                    <hr>
                    <button sfx="button">
                        <span>Export Spells</span>
                    </button>
                    <button sfx="button">
                        <span>Export Monsters</span>
                    </button>
                    <hr>
                    <button sfx="button">
                        <span>Options</span>
                        <span>Ctrl+,</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.clickExit}>
                        <span>Exit</span>
                        <span>Alt+F4</span>
                    </button>
                </div>
            `:n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Export Spells</span>
                    </button>
                    <button sfx="button">
                        <span>Export Monsters</span>
                    </button>
                    <hr>
                    <button sfx="button">
                        <span>Options</span>
                        <span>Ctrl+,</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.clickExit}>
                        <span>Exit</span>
                        <span>Alt+F4</span>
                    </button>
                </div>
            `}renderTabletopMenu(){return sessionStorage.getItem("role")==="gm"?n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button" @click=${this.clickLoadImage}>
                        <span>Load image</span>
                        <span>Ctrl+N</span>
                    </button>
                    <button sfx="button" @click=${this.spawnPawns}>
                        <span>Spawn pawns</span>
                        <span>Ctrl+L</span>
                    </button>
                    <button sfx="button" @click=${this.clearImage}>
                        <span>Clear tabletop</span>
                        <span>Ctrl+Backspace</span>
                    </button>
                </div>
            `:""}renderWindowsMenu(){return sessionStorage.getItem("role")==="gm"?n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Initiative tracker</span>
                    </button>
                    <button sfx="button">
                        <span>Chat</span>
                    </button>
                    <button sfx="button" @click=${this.openMonsterManual}>
                        <span>Monster Manual</span>
                    </button>
                    <button sfx="button" @click=${this.openSpellbook}>
                        <span>Spellbook</span>
                    </button>
                    <button sfx="button">
                        <span>Dice tray</span>
                    </button>
                    <button sfx="button">
                        <span>Drawing tools</span>
                    </button>
                    <button sfx="button">
                        <span>Music</span>
                    </button>
                </div>
            `:n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Initiative tracker</span>
                    </button>
                    <button sfx="button">
                        <span>Chat</span>
                    </button>
                    <button sfx="button">
                        <span>Spellbook</span>
                    </button>
                    <button sfx="button">
                        <span>Dice tray</span>
                    </button>
                    <button sfx="button">
                        <span>Drawing tools</span>
                    </button>
                </div>
            `}renderViewMenu(){return n`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button" @click=${this.zoomIn}>
                    <span>Zoom in</span>
                    <span>Ctrl+=</span>
                </button>
                <button sfx="button" @click=${this.zoomOut}>
                    <span>Zoom out</span>
                    <span>Ctrl+-</span>
                </button>
                <button sfx="button" @click=${this.zoomReset}>
                    <span>100%</span>
                    <span>Ctrl+0</span>
                </button>
                <button sfx="button" @click=${this.zoom200}>
                    <span>200%</span>
                    <span>Ctrl+0</span>
                </button>
                <hr>
                <button sfx="button" @click=${this.toggleFullscreen}>
                    <span>Toggle fullscreen</span>
                    <span>F11</span>
                </button>
                <button sfx="button" @click=${this.centerTabletop}>
                    <span>Center tabletop</span>
                </button>
            </div>
        `}renderHelpMenu(){return n`
            <div style="left:${this.calcOffsetX()}px;" class="menu">
                <button sfx="button">
                    <span>Report issue</span>
                </button>
                <button sfx="button">
                    <span>Show user guides</span>
                </button>
                <button sfx="button">
                    <span>Show keyboard shortcuts</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>Privacy policy</span>
                </button>
                <button sfx="button">
                    <span>License</span>
                </button>
                <hr>
                <button sfx="button">
                    <span>About</span>
                </button>
            </div>
        `}renderInitiativeMenu(){return sessionStorage.getItem("role")==="gm"?n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button">
                        <span>Next</span>
                        <span>Ctrl+Shift+=</span>
                    </button>
                    <button sfx="button">
                        <span>Previous</span>
                        <span>Ctrl+Shift+-</span>
                    </button>
                    <hr>
                    <button sfx="button">
                        <span>Clear tracker</span>
                        <span>Ctrl+Shift+Backspace</span>
                    </button>
                    <button sfx="button">
                        <span>Sync tracker</span>
                        <span>Ctrl+R</span>
                    </button>
                </div>
            `:""}renderRoomMenu(){return sessionStorage.getItem("role")==="gm"?n`
                <div style="left:${this.calcOffsetX()}px;" class="menu">
                    <button sfx="button" @click=${this.lockRoom}>
                        <span>Lock room</span>
                    </button>
                    <button sfx="button" @click=${this.unlockRoom}>
                        <span>Unlock room</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.copyRoomCode}>
                        <span>Copy room code</span>
                    </button>
                    <hr>
                    <button sfx="button" @click=${this.openPlayerMenu}>
                        <span>Player list</span>
                    </button>
                </div>
            `:""}render(){let t;switch(this.model.menu){case"room":t=this.renderRoomMenu();break;case"window":t=this.renderWindowsMenu();break;case"file":t=this.renderFileMenu();break;case"tabletop":t=this.renderTabletopMenu();break;case"initiative":t=this.renderInitiativeMenu();break;case"help":t=this.renderHelpMenu();break;case"view":t=this.renderViewMenu();break;default:break}const e=n`
            <div @click=${this.clickBackdrop} class="backdrop"></div>
            ${t}
        `;u(e,this)}}i.bind("tool-bar-menu",r);export{r as default};
