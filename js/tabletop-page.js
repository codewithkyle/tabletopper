import r from"./jsql.js";import{navigateTo as m}from"./router.js";import l from"./supercomponent.js";import{html as n,render as p}from"./lit-html.js";import c from"./spinner.js";import a from"./env.js";import g from"./control-center.js";import{send as d}from"./ws.js";import u from"./tabletop-component.js";import b from"./tool-bar.js";class i extends l{constructor(e,t){super();const o=sessionStorage.getItem("room"),s=sessionStorage.getItem("lastSocketId");o===e.CODE.toUpperCase()&&s!==sessionStorage.getItem("socketId")?d("core:sync",{room:o,prevId:s}):m("/")}async connected(){await a.css(["tabletop-page"]),this.setAttribute("state","SYNCING"),this.render(),await r.query("RESET ledger"),await r.query("INSERT INTO games VALUES ($game)",{game:{map:null,room:sessionStorage.getItem("room")}});const{SERVER_URL:e}=await import("/config.js"),t=`${e.trim().replace(/\/$/,"")}`;(await fetch(`${t}/room/${sessionStorage.getItem("room")}`,{method:"HEAD"})).status===200&&(await r.ingest(`${t}/room/${sessionStorage.getItem("room")}`,"ledger","NDJSON"),g.runHistory()),this.setAttribute("state","IDLING")}renderSyncing(){return n`
            <div class="absolute center spinner">
                ${new c({size:32,class:"mb-0.75 mx-auto block"})}
                <span class="block text-center font-xs font-grey-700">Synchronizing game state...</span>
            </div>
        `}async render(){const e=n`
            ${new b}
            ${this.renderSyncing()}
            ${new u}
        `;p(e,this)}}a.bind("tabletop-page",i);export{i as default};
