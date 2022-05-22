import r from"./jsql.js";import{navigateTo as m}from"./router.js";import p from"./supercomponent.js";import{html as a,render as c}from"./lit-html.js";import l from"./spinner.js";import n from"./env.js";import g from"./control-center.js";import{send as d}from"./ws.js";import u from"./tabletop-component.js";import f from"./tool-bar.js";class i extends p{constructor(e,t){super();const o=sessionStorage.getItem("room"),s=sessionStorage.getItem("lastSocketId");o===e.CODE.toUpperCase()&&s!==sessionStorage.getItem("socketId")?d("core:sync",{room:o,prevId:s}):m("/")}async connected(){await n.css(["tabletop-page"]),this.setAttribute("state","SYNCING"),this.render(),await r.query("RESET ledger"),await r.query("INSERT INTO games VALUES ($game)",{game:{map:null,room:sessionStorage.getItem("room")}});const{SERVER_URL:e}=await import("/config.js"),t=`${e.trim().replace(/\/$/,"")}`;(await fetch(`${t}/room/${sessionStorage.getItem("room")}`,{method:"HEAD"})).status===200&&(await r.ingest(`${t}/room/${sessionStorage.getItem("room")}`,"ledger","NDJSON"),g.runHistory()),this.setAttribute("state","IDLING")}renderSyncing(){return a`
            <div class="absolute center spinner">
                ${new l({size:32,class:"mb-0.75 mx-auto block"})}
                <span class="block text-center font-xs font-grey-700">Synchronizing game state...</span>
            </div>
        `}async render(){const e=a`
            ${new f}
            ${this.renderSyncing()}
            ${new u}
        `;c(e,this)}}n.bind("tabletop-page",i);export{i as default};
