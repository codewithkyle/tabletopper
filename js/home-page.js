import{subscribe as u}from"./pubsub.js";import{navigateTo as c}from"./router.js";import h from"./supercomponent.js";import{render as g,html as t}from"./lit-html.js";import o from"./button.js";import d from"./input.js";import b from"./spinner.js";import w from"./env.js";import k from"./notifications.js";import m from"./control-center.js";import f from"./disk-jockey.js";import{connected as v,send as s}from"./ws.js";import a from"./homepage-music-player.js";import y from"./player-token-picker.js";class x extends h{constructor(){super();this.state="WELCOME",this.stateMachine={WELCOME:{NEXT:"MENU"},MENU:{JOIN:"ROOM"},ROOM:{NEXT:"CHARACTER",BACK:"MENU"},CHARACTER:{BACK:"ROOM"}},this.model={connected:v,code:""},sessionStorage.getItem("kicked")&&(k.error("Kicked From Game","You have been kicked by the Game Master."),sessionStorage.removeItem("kicked")),u("socket",this.inbox.bind(this))}inbox({type:e,data:r}){switch(e){case"connected":this.model.connected||this.set({connected:!0});break;case"disconnected":this.model.connected&&this.set({connected:!1});break;case"room:join":c(`/room/${r.code}`),f.pause("mainMenu");break;case"character:getDetails":this.trigger("NEXT");break;default:break}}async connected(){await w.css(["homepage"]),this.render()}renderActions(){return t`
            <div class="actions w-full" flex="row nowrap items-center justify-center">
                ${new o({callback:()=>{this.trigger("JOIN")},label:"Join Room",kind:"outline",color:"white",class:"mr-1",size:"large",icon:'<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-login" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><path d="M14 8v-2a2 2 0 0 0 -2 -2h-7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h7a2 2 0 0 0 2 -2v-2"></path><path d="M20 12h-13l3 -3m0 6l-3 -3"></path></svg>'})}
                ${new o({callback:()=>{s("create:room"),sessionStorage.setItem("role","gm")},label:"Create Room",kind:"outline",color:"white",size:"large",icon:'<svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-circle-plus" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round"><path stroke="none" d="M0 0h24v24H0z" fill="none"></path><circle cx="12" cy="12" r="9"></circle><line x1="9" y1="12" x2="15" y2="12"></line><line x1="12" y1="9" x2="12" y2="15"></line></svg>'})}
                </div>
            </div> 
       `}renderLoading(){return t`
            <div flex="items-center justify-center row nowrap" class="actions w-full" style="height:42px;">
                ${new b({size:24,color:"white"})}
                <span class="ml-0.5 font-lg font-medium font-white">Connecting to online services...</span>
            </div>
       `}renderWelcome(){return t`
            <div class="w-768 bg-white border-1 border-solid border-grey-300 shadow-grey-sm radius-0.5 no-scroll">
                <h1 class="px-2 pt-1.75 font-grey-900 font-2xl line-normal font-bold block">Greetings Adventurer!</h1>
                <p class="px-2 pb-2 pt-1 font-grey-700 line-normal block">
                    Welcome to the alpha build of Tabletopper, a free web-based virtual tabletop (VTT). Before you continue we must warn you that this is an alpha build which means you may experience some bugs or glitches. Please report any issues through the help menu.
                    <br>
                    <br>
                    May your adventures be grand and your rewards bountiful.
                </p>
                <div flex="justify-end items-center row nowrap" class="p-1 bg-grey-100 border-t-1 border-t-solid border-t-grey-200">
                    ${new o({kind:"solid",color:"info",callback:()=>{this.trigger("NEXT")},label:"I understand"})}
                </div>
            </div>
       `}renderMenu(){return t`
            <div class="w-1024 menu text-center">
                <h1>Tabletopper</h1>
                ${this.model.connected?this.renderActions():this.renderLoading()}
            </div>
            ${new a}
       `}renderJoin(){return t`
            <div class="join w-411 bg-white border-1 border-solid border-grey-300 shadow-grey-sm radius-0.5 no-scroll">
                <div class="p-1.5">
                    ${new d({name:"room",label:"Room Code",value:this.model.code,maxlength:4,minlength:4,required:!0})}
                </div>
                <div flex="row nowrap items-center justify-end" class="p-1 bg-grey-100 border-t-1 border-t-solid border-t-grey-300">
                    ${new o({label:"Back",class:"mr-1",callback:()=>{this.trigger("BACK")}})}
                    ${new o({label:"Next",callback:()=>{const e=this.querySelector(".js-input"),r=e.getValue().toString().trim().toUpperCase();if(e.validate()){const i=sessionStorage.getItem("room"),n=sessionStorage.getItem("lastSocketId");i===r&&n!=null?c(`/room/${r}`):s("room:check",{code:r})}}})}
                </div>
            </div>
            ${new a}
       `}renderCharacter(){return t`
            <div class="join w-411 bg-white border-1 border-solid border-grey-300 shadow-grey-sm radius-0.5 no-scroll">
                <div class="p-1.5" flex="row nowrap items-center">
                    ${new y}
                    ${new d({class:"ml-1.5 mb-0.25",name:"name",label:"Character Name",required:!0,css:"flex:1;"})}
                </div>
                <div flex="row nowrap items-center justify-end" class="p-1 bg-grey-100 border-t-1 border-t-solid border-t-grey-300">
                    ${new o({label:"Back",class:"mr-1",callback:()=>{this.trigger("BACK")}})}
                    ${new o({label:"Join Room",kind:"solid",color:"primary",callback:async()=>{const e=this.querySelector(".js-input"),r=this.querySelector("player-token-picker");if(e.validate()){const i=await r.getValue();let n=null;i&&(n=m.insert("images",i.uid,i));const l=e.getValue().toString(),p=m.insert("players",sessionStorage.getItem("socketId"),{uid:sessionStorage.getItem("socketId"),name:l,token:i?.uid??null,x:0,y:0,active:!0});s("room:join",{name:l,player:p,token:n}),sessionStorage.setItem("role","player")}}})}
                </div>
            </div>
            ${new a}
       `}render(){let e;switch(this.state){case"WELCOME":e=this.renderWelcome();break;case"MENU":e=this.renderMenu();break;case"ROOM":e=this.renderJoin();break;case"CHARACTER":e=this.renderCharacter();break}const r=t`
            <img src="/images/background.jpg" width="1920" loading="lazy" draggable="false">
            <div class="w-full h-full" flex="justify-center items-center column wrap">
                ${e}
            </div>
        `;setTimeout(()=>{g(r,this)},100)}}export{x as default};
