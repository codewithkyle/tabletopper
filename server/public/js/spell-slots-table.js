import h from"./supercomponent.js";import{html as p,render as g}from"./lit-html.js";import{unsafeHTML as $}from"./unsafe-html.js";import v from"./env.js";import{parseDataset as S}from"./general.js";const m=["Abjuration","Conjuration","Divination","Enchantment","Evocation","Illusion","Necromancy","Transmutation"],u=[0,1,2,3,4,5,6,7,8,9],b=r=>r===0?"Cantrips":`Level ${r}`,y=()=>({name:"",components:"",school:"Evocation",castingTime:"",range:"",duration:"",text:""}),d=(r,c=0,s=99)=>{if(!Number.isFinite(r))return c;const e=Math.trunc(r);return Math.max(c,Math.min(s,e))};class f extends h{constructor(){super();this.noopEvent=s=>{s.stopImmediatePropagation()};this.updateSlots=s=>{const e=s.currentTarget,t=parseInt(e.dataset.level),l=e.value.trim(),n=l===""?0:d(Number(l)),o=this.get(),a=String(t),i=o.levels[a];i.slots=n,i.used=d(i.used,0,i.slots),this.set(o,!0)};this.updateUsed=s=>{const e=s.currentTarget,t=parseInt(e.dataset.level),l=e.value.trim(),n=l===""?0:d(Number(l)),o=this.get(),a=String(t),i=o.levels[a];i.used=d(n,0,i.slots),this.set(o,!0)};this.addSpell=s=>{const e=s.currentTarget,t=parseInt(e.dataset.level),l=this.get();l.levels[String(t)].spells.push(y()),this.set(l)};this.deleteSpell=s=>{const e=s.currentTarget,t=parseInt(e.dataset.level),l=parseInt(e.dataset.index),n=this.get();n.levels[String(t)].spells.splice(l,1),this.set(n)};this.updateSpellField=s=>{const e=s.currentTarget,t=parseInt(e.dataset.level||"0"),l=parseInt(e.dataset.index||"0"),n=e.dataset.field,o=this.get(),a=o.levels[String(t)].spells[l];a&&(a[n]=e.value,this.set(o,!0))};const s={};u.forEach(e=>{s[String(e)]={level:e,slots:0,used:0,spells:[]}}),this.model={label:"Spell Slots",name:"spells",addLabel:"Add Spell",levels:s}}static get observedAttributes(){return["data-label","data-name","data-add-label","data-levels"]}async connected(){await v.css(["spell-slots-table"]);const s=S(this.dataset,this.model),e=s.levels&&typeof s.levels=="object"&&!Array.isArray(s.levels)?s.levels:{},t={...this.model,...s,levels:{...this.model.levels||{},...e}};u.forEach(l=>{const n=String(l),o=t.levels[n];t.levels[n]={level:l,slots:d(o?.slots??0),used:d(o?.used??0),spells:Array.isArray(o?.spells)?o.spells.map(a=>({name:typeof a?.name=="string"?a.name:"",components:typeof a?.components=="string"?a.components:"",school:typeof a?.school=="string"&&m.includes(a.school)?a.school:"Evocation",castingTime:typeof a?.castingTime=="string"?a.castingTime:"",range:typeof a?.range=="string"?a.range:"",duration:typeof a?.duration=="string"?a.duration:"",text:typeof a?.text=="string"?a.text:""})):[]}}),this.set(t)}renderSpellLevels(s,e){return s.spells.length==0?"":p`
            <spells-list>
                ${s.spells.map((t,l)=>{const n=`${this.model.name}-level-${e}-spell-${l}`;return p`
                        <spell-card>
                            <spell-top>
                                <div class="name">
                                    <input
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        type="text"
                                        required
                                        placeholder="Spell name"
                                        name="${n}-name"
                                        .value="${t.name}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="name"
                                        @input=${this.updateSpellField}
                                    />
                                </div>

                                <button
                                    class="delete"
                                    type="button"
                                    aria-label="Delete ${t.name||"spell"}"
                                    tooltip
                                    data-level="${e}"
                                    data-index="${l}"
                                    @click=${this.deleteSpell}
                                >
                                    <svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512">
                                        <path fill="currentColor" d="M296 432h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zm-160 0h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zM440 64H336l-33.6-44.8A48 48 0 0 0 264 0h-80a48 48 0 0 0-38.4 19.2L112 64H8a8 8 0 0 0-8 8v16a8 8 0 0 0 8 8h24v368a48 48 0 0 0 48 48h288a48 48 0 0 0 48-48V96h24a8 8 0 0 0 8-8V72a8 8 0 0 0-8-8zM171.2 38.4A16.1 16.1 0 0 1 184 32h80a16.1 16.1 0 0 1 12.8 6.4L296 64H152zM384 464a16 16 0 0 1-16 16H80a16 16 0 0 1-16-16V96h320zm-168-32h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8z"></path>
                                    </svg>
                                </button>
                            </spell-top>

                            <spell-grid>
                                <label class="full">
                                    <span class="k">Components</span>
                                    <input
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        type="text"
                                        placeholder="V, S, M (a bit of sponge)"
                                        name="${n}-components"
                                        .value="${t.components}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="components"
                                        @input=${this.updateSpellField}
                                    />
                                </label>

                                <label>
                                    <span class="k">School</span>
                                    <select
                                        name="${n}-school"
                                        .value="${t.school}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="school"
                                        @change=${this.updateSpellField}
                                    >
                                        ${m.map(o=>p`<option value="${o}">${o}</option>`)}
                                    </select>
                                    <i class="selector">
                                        <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
                                        </svg>
                                    </i>
                                </label>

                                <label>
                                    <span class="k">Cast Time</span>
                                    <input
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        type="text"
                                        placeholder="Action"
                                        name="${n}-castingTime"
                                        .value="${t.castingTime}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="castingTime"
                                        @input=${this.updateSpellField}
                                    />
                                </label>

                                <label>
                                    <span class="k">Range</span>
                                    <input
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        type="text"
                                        placeholder="150 feet"
                                        name="${n}-range"
                                        .value="${t.range}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="range"
                                        @input=${this.updateSpellField}
                                    />
                                </label>

                                <label>
                                    <span class="k">Duration</span>
                                    <input
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        type="text"
                                        placeholder="Instantaneous"
                                        name="${n}-duration"
                                        .value="${t.duration}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="duration"
                                        @input=${this.updateSpellField}
                                    />
                                </label>

                                <label class="full">
                                    <span class="k">Spell Text</span>
                                    <textarea
                                        @keydown=${this.noopEvent}
                                        @keyup=${this.noopEvent}
                                        rows="5"
                                        placeholder="Describe the spell..."
                                        name="${n}-text"
                                        .value="${t.text}"
                                        data-level="${e}"
                                        data-index="${l}"
                                        data-field="text"
                                        @input=${this.updateSpellField}
                                    >${t.text}</textarea>
                                </label>
                            </spell-grid>
                        </spell-card>
                    `})}
            </spells-list>
        `}render(){const s=typeof this.model.label=="string"?this.model.label.trim():"",e=p`
            ${s?p`<h4 class="block w-full font-medium font-sm font-grey-800 dark:font-grey-300 pl-0.125">
                      ${$(s)}
                  </h4>`:null}

            <levels-grid>
                ${u.map(t=>{const l=this.model.levels[String(t)],n=`${this.model.name}-level-${t}-slots`,o=`${this.model.name}-level-${t}-used`;return p`
                        <spell-level>
                            <level-header>
                                <div class="title">
                                    <span class="lvl">${b(t)}</span>
                                </div>

                                <div class="slots">
                                    <label>
                                        <span class="k">Slots</span>
                                        <input
                                            @keydown=${this.noopEvent}
                                            @keyup=${this.noopEvent}
                                            type="number"
                                            step="1"
                                            min="0"
                                            name="${n}"
                                            .value="${String(l.slots??0)}"
                                            data-level="${t}"
                                            @input=${this.updateSlots}
                                        />
                                    </label>

                                    <label>
                                        <span class="k">Used</span>
                                        <input
                                            @keydown=${this.noopEvent}
                                            @keyup=${this.noopEvent}
                                            type="number"
                                            step="1"
                                            min="0"
                                            name="${o}"
                                            .value="${String(l.used??0)}"
                                            data-level="${t}"
                                            @input=${this.updateUsed}
                                        />
                                    </label>
                                </div>
                            </level-header> 
                            ${this.renderSpellLevels(l,t)}
                            <button
                                type="button"
                                class="bttn"
                                kind="dashed"
                                dull
                                color="warning"
                                data-level="${t}"
                                @click=${this.addSpell}
                            >
                                ${this.model.addLabel}
                            </button>
                        </spell-level>
                    `})}
            </levels-grid>
        `;g(e,this)}}v.bind("spell-slots-table",f);
