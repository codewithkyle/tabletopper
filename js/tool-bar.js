import{subscribe as r}from"./pubsub.js";import c from"./supercomponent.js";import{html as u,render as d}from"./lit-html.js";import i from"./env.js";import a from"./tool-bar-menu.js";class l extends c{constructor(){super();this.handleClick=e=>{const o=e.currentTarget,t=o.dataset.menu;this.querySelectorAll("button[data-menu]").forEach(s=>{s.classList.remove("is-open")}),o.classList.add("is-open"),this.menuOpen=!0;const n=document.body.querySelector("tool-bar-menu")||new a(t);n.isConnected?n.changeMenu(t):document.body.append(n)};this.handleMouseEnter=e=>{if(this.menuOpen){const o=e.currentTarget,t=o.dataset.menu;this.querySelectorAll("button[data-menu]").forEach(s=>{s.classList.remove("is-open")}),o.classList.add("is-open");const n=document.body.querySelector("tool-bar-menu")||new a(t);n.isConnected?n.changeMenu(t):document.body.append(n)}};this.menuOpen=!1,r("toolbar",this.inbox.bind(this))}inbox({type:e,data:o}){switch(e){case"menu:close":this.menuOpen=!1,this.querySelectorAll("button[data-menu]").forEach(t=>{t.classList.remove("is-open")});break;default:break}}async connected(){await i.css(["tool-bar"]),this.render()}render(){let e;sessionStorage.getItem("role")==="gm"?e=u`
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="file">File</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="room">Room</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="tabletop">Tabletop</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="initiative">Initiative</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="view">View</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="window">Window</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="help">Help</button>
            `:e=u`
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="file">File</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="view">View</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="window">Window</button>
                <button sfx="button" @mouseenter=${this.handleMouseEnter} @click=${this.handleClick} data-menu="help">Help</button>
            `,d(e,this)}}i.bind("tool-bar",l);export{l as default};
