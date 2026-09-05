import l from"./supercomponent.js";import{html as r,render as u}from"./lit-html.js";import{unsafeHTML as m}from"./unsafe-html.js";import i from"./env.js";import{parseDataset as v}from"./general.js";const d=[{key:"str",label:"Strength"},{key:"dex",label:"Dexterity"},{key:"con",label:"Constitution"},{key:"int",label:"Intelligence"},{key:"wis",label:"Wisdom"},{key:"cha",label:"Charisma"}];class p extends l{constructor(){super();this.noopEvent=e=>{e.stopImmediatePropagation()};this.updateBonus=e=>{const s=e.currentTarget,o=s.dataset.key,t=s.value.trim(),n=t===""?0:Number(t),a=this.get();a.values={...a.values||{}},a.values[o]=Number.isFinite(n)?n:0,this.set(a,!0)};this.model={label:"Saving Throws",name:"saves",values:{}}}static get observedAttributes(){return["data-label","data-name","data-values"]}async connected(){await i.css(["saving-throws-table"]);const e=v(this.dataset,this.model);e.values=e.values??{},this.set(e)}render(){const e=this.model.values||{},s=typeof this.model.label=="string"?this.model.label.trim():"",o=r`
            ${s?r`<h4 class="m-0 block w-full pl-0.5 text-[0.71rem] font-bold tracking-[0.08em] uppercase text-base-content/75">
                      ${m(s)}
                  </h4>`:null}

            <saves-grid>
                ${d.map(t=>{const n=e[t.key]??0,a=`${this.model.name}-${t.key}`;return r`
                        <save-row>
                            <save-name>
                                <span class="label">${t.label}</span>
                                <span class="meta">${t.key.toUpperCase()}</span>
                            </save-name>

                            <input
                                @keydown=${this.noopEvent}
                                @keyup=${this.noopEvent}
                                class="bonus"
                                type="number"
                                inputmode="numeric"
                                step="1"
                                name="${a}"
                                aria-label="${t.label} saving throw bonus"
                                .value="${String(n)}"
                                data-key="${t.key}"
                                @input=${this.updateBonus}
                            />
                        </save-row>
                    `})}
            </saves-grid>
        `;u(o,this)}}i.bind("saving-throws-table",p);
