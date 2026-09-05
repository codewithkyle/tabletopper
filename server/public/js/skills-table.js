import r from"./supercomponent.js";import{html as s,render as b}from"./lit-html.js";import{unsafeHTML as y}from"./unsafe-html.js";import o from"./env.js";import{parseDataset as u}from"./general.js";const c=[{key:"acrobatics",label:"Acrobatics",ability:"DEX"},{key:"animal_handling",label:"Animal Handling",ability:"WIS"},{key:"arcana",label:"Arcana",ability:"INT"},{key:"athletics",label:"Athletics",ability:"STR"},{key:"deception",label:"Deception",ability:"CHA"},{key:"history",label:"History",ability:"INT"},{key:"insight",label:"Insight",ability:"WIS"},{key:"intimidation",label:"Intimidation",ability:"CHA"},{key:"investigation",label:"Investigation",ability:"INT"},{key:"medicine",label:"Medicine",ability:"WIS"},{key:"nature",label:"Nature",ability:"INT"},{key:"perception",label:"Perception",ability:"WIS"},{key:"performance",label:"Performance",ability:"CHA"},{key:"persuasion",label:"Persuasion",ability:"CHA"},{key:"religion",label:"Religion",ability:"INT"},{key:"sleight_of_hand",label:"Sleight of Hand",ability:"DEX"},{key:"stealth",label:"Stealth",ability:"DEX"},{key:"survival",label:"Survival",ability:"WIS"}];class m extends r{constructor(){super();this.noopEvent=i=>{i.stopImmediatePropagation()};this.updateBonus=i=>{const e=i.currentTarget,n=e.dataset.key,t=e.value.trim(),l=t===""?0:Number(t),a=this.get();a.values={...a.values||{}},a.values[n]=Number.isFinite(l)?l:0,this.set(a,!0)};this.model={label:"Skills",name:"skills",values:{}}}static get observedAttributes(){return["data-label","data-name","data-values"]}async connected(){await o.css(["skills-table"]);const i=u(this.dataset,this.model),e=i.values;i.values=e&&typeof e=="object"&&!Array.isArray(e)?e:{},this.set(i)}render(){const i=this.model.values||{},e=typeof this.model.label=="string"?this.model.label.trim():"",n=s`
            ${e?s`<h4 class="m-0 block w-full pl-0.5 text-[0.71rem] font-bold tracking-[0.08em] uppercase text-base-content/75">
                      ${y(e)}
                  </h4>`:null}

            <skills-grid>
                ${c.map(t=>{const l=i[t.key]??0,a=`${this.model.name}-${t.key}`;return s`
                        <skill-row>
                            <skill-name>
                                <span class="label">${t.label}</span>
                                <span class="ability">${t.ability}</span>
                            </skill-name>

                            <input
                                @keydown=${this.noopEvent}
                                @keyup=${this.noopEvent}
                                class="bonus"
                                type="number"
                                inputmode="numeric"
                                step="1"
                                name="${a}"
                                aria-label="${t.label} bonus"
                                .value="${String(l)}"
                                data-key="${t.key}"
                                @input=${this.updateBonus}
                            />
                        </skill-row>
                    `})}
            </skills-grid>
        `;b(n,this)}}o.bind("skills-table",m);
