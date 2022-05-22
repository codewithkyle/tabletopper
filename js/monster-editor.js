import d from"./jsql.js";import{publish as p}from"./pubsub.js";import m from"./supercomponent.js";import{html as u,render as c}from"./lit-html.js";import n from"./input.js";import i from"./number-input.js";import h from"./select.js";import r from"./env.js";import w from"./notifications.js";class v extends m{constructor(e=null){super();this.saveCreature=async e=>{e.preventDefault(),e.stopImmediatePropagation();const t=e.currentTarget,l={};let a=!0;t.querySelectorAll(".js-input").forEach(s=>{s.validate()?l[s.getName()]=s.getValue():a=!1}),a&&(this.model.index!==null?l.index=this.model.index:(l.index=l.name.replace(/[^a-z\s]/gi,"").trim().replace(/\s+/g,"-"),this.set({index:l.index},!0)),await d.query("INSERT INTO monsters VALUES ($monster)",{monster:l}),w.snackbar(`${l.name} has been updated.`),p(l.index,l))};this.model={index:e,name:null,size:null,type:null,subtype:null,alignment:null,ac:null,hp:null,hitDice:null,str:null,dex:null,con:null,int:null,wis:null,cha:null,languages:null,cr:null,xp:null,speed:null,vulnerabilities:null,resistances:null,immunities:null,senses:null,savingThrows:null,skills:null,abilities:null,actions:null,legendaryActions:null}}async connected(){await r.css(["monster-editor"]);const e=(await d.query("SELECT * FROM monsters WHERE index = $index",{index:this.model.index}))?.[0]??null;e!==null&&this.set(e,!0),this.render()}render(){const e=[{label:"Tiny",value:"Tiny"},{label:"Small",value:"Small"},{label:"Medium",value:"Medium"},{label:"Large",value:"Large"},{label:"Huge",value:"Huge"},{label:"Gargantuan",value:"Gargantuan"}],t=[{label:"Unaligned",value:"unaliged"},{label:"Any Alignment",value:"any alignment"},{label:"Lawful Good",value:"lawful good"},{label:"Neutral Good",value:"neutral good"},{label:"Chaotic Good",value:"chaotic good"},{label:"Lawful Neutral",value:"lawful neutral"},{label:"Neutral",value:"neutral"},{label:"Chaotic Neutral",value:"chaotic neutral"},{label:"Lawful Evil",value:"lawful evil"},{label:"Neutral Evil",value:"neutral evil"},{label:"Chaotic Evil",value:"chaotic evil"},{label:"Any Neutral Alignment",value:"any neutral alignment"},{label:"Any Good Alignment",value:"any good alignment"},{label:"Any Chaotic Alignment",value:"any chaotic alignment"},{label:"Any Lawful Alignment",value:"any lawful alignment"},{label:"Any Non-Good Alignment",value:"any non-good alignment"}],l=["aberration","beast","celestial","construct","dragon","elemental","fey","fiend","giant","humanoid","monstrositie","ooze","plant","undead","swam of tiny beasts"],a=["any race","demon","devil","dwarf","elf","gnoll","gnome","goblinoid","grimlock","human","kobold","lizardfolk","merfolk","orc","sahuagin","shapechanger","titan","angel","swarm"],s=u`
            <form @submit=${this.saveCreature} class="w-full p-1" grid="columns 1 gap-1">
                <div class="w-full" flex="row nowrap items-center">
                    ${new n({name:"name",label:"Name",value:this.model.name||"Unnamed Monster",required:!0,css:"flex:1;"})}
                </div>
                <div grid="columns 2 gap-1">
                    ${new h({name:"size",label:"Size",value:this.model.size,options:e,required:!0})}
                    ${new h({name:"alignment",label:"Alignment",value:this.model.alignment,options:t,required:!0})}
                    ${new n({label:"Type",name:"type",value:this.model.type,required:!0,datalist:l})}
                    ${new n({label:"Subtype",name:"subtype",value:this.model.subtype,datalist:a})}
                    ${new i({label:"Armor Class",name:"ac",value:this.model.ac,required:!0})}
                    ${new i({label:"Hit Points",name:"hp",value:this.model.hp,required:!0})}
                </div>
                ${new n({label:"Hit Dice",name:"hitDice",value:this.model.hitDice,required:!0})}
                ${new n({label:"Speed",name:"speed",value:this.model.speed,required:!0})}
                <div grid="columns 2 gap-1">
                    ${new i({label:"Challenge Rating",name:"cr",value:this.model.cr,required:!0})}
                    ${new i({label:"XP",name:"xp",value:this.model.xp,required:!0,min:0,max:9999999})}
                </div>
                <div grid="columns 3 gap-1">
                    ${new i({label:"Strength",name:"str",value:this.model.str,required:!0})}
                    ${new i({label:"Dexterity",name:"dex",value:this.model.dex,required:!0})}
                    ${new i({label:"Constitution",name:"con",value:this.model.con,required:!0})}
                    ${new i({label:"Intelligence",name:"int",value:this.model.int,required:!0})}
                    ${new i({label:"Wisdom",name:"wis",value:this.model.wis,required:!0})}
                    ${new i({label:"Charisma",name:"cha",value:this.model.cha,required:!0})}
                </div>
                ${new n({label:"Immunities",name:"immunities",value:this.model.immunities})}
                ${new n({label:"Resistances",name:"resistances",value:this.model.resistances})}
                ${new n({label:"Vulnerabilities",name:"vulnerabilities",value:this.model.vulnerabilities})}
                ${new n({label:"Senses",name:"senses",value:this.model.senses})}
                ${new n({label:"Languages",name:"languages",value:this.model.languages})}
                ${new n({label:"Saving Throws",name:"savingThrows",value:this.model.savingThrows})}
                ${new n({label:"Skills",name:"skills",value:this.model.skills})}
                ${new o({label:"Abilities",name:"abilities",rows:this.model.abilities||[],addLabel:"Add Ability",class:"mt-1"})}
                ${new o({label:"Actions",name:"actions",rows:this.model.actions||[],addLabel:"Add Action",class:"mt-1"})}
                ${new o({label:"Legendary Actions",name:"legendaryActions",rows:this.model.legendaryActions||[],addLabel:"Add Legendary Action",class:"mt-1"})}
                <div class="w-full">
                    <button class="bttn w-full" kind="solid" color="success">Save monster</button>
                </div>
            </form>
        `;c(s,this)}}r.bind("monster-editor",v);class o extends m{constructor(e={}){super();this.addRow=()=>{const e=this.get();e.rows.push({name:"",desc:""}),this.set(e)};this.deleteRow=e=>{const t=e.currentTarget,l=parseInt(t.dataset.index),a=this.get();a.rows.splice(l,1),this.set(a)};this.updateName=e=>{const t=e.currentTarget,l=parseInt(t.dataset.index),a=this.get();a.rows[l].name=t.value,this.set(a,!0)};this.updateDesc=e=>{const t=e.currentTarget,l=parseInt(t.dataset.index),a=this.get();a.rows[l].desc=t.value,this.set(a,!0)};this.model={label:"",rows:[],name:"",addLabel:"Add Row",class:"",css:""},this.model=Object.assign(this.model,e)}async connected(){await r.css(["monster-info-table","buttons"]),this.render()}validate(){return!0}getName(){return this.model.name}getValue(){const e=[],t=Array.from(this.querySelectorAll("table-row"));for(let l=0;l<t.length;l++){const a=t[l].querySelector("input"),s=t[l].querySelector("textarea");e.push({name:a.value,desc:s.value})}return e}render(){console.log(this.model),this.style.cssText=this.model.css,this.className=`${this.model.class} js-input`;const e=u`
            <h4 class="block w-full font-medium font-sm font-grey-800 pl-0.125">${this.model.label}</h4>
            ${this.model.rows.map((t,l)=>u`
                    <table-row>
                        <row-title>
                            <input required value="${t.name}" placeholder="Name" @input=${this.updateName} data-index="${l}">
                            <button aria-label="Delete ${t.name}" tooltip class="delete" type="button" @click=${this.deleteRow} data-index="${l}">
                                <svg aria-hidden="true" focusable="false" role="img" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path fill="currentColor" d="M296 432h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zm-160 0h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8zM440 64H336l-33.6-44.8A48 48 0 0 0 264 0h-80a48 48 0 0 0-38.4 19.2L112 64H8a8 8 0 0 0-8 8v16a8 8 0 0 0 8 8h24v368a48 48 0 0 0 48 48h288a48 48 0 0 0 48-48V96h24a8 8 0 0 0 8-8V72a8 8 0 0 0-8-8zM171.2 38.4A16.1 16.1 0 0 1 184 32h80a16.1 16.1 0 0 1 12.8 6.4L296 64H152zM384 464a16 16 0 0 1-16 16H80a16 16 0 0 1-16-16V96h320zm-168-32h16a8 8 0 0 0 8-8V152a8 8 0 0 0-8-8h-16a8 8 0 0 0-8 8v272a8 8 0 0 0 8 8z"></path></svg>
                            </button>
                        </row-title>
                        <textarea required placeholder="Description" rows="4" @input=${this.updateDesc} data-index="${l}">${t.desc}</textarea>
                    </table-row>
                `)}
            <button type="button" class="bttn" kind="text" color="grey" @click=${this.addRow}>${this.model.addLabel}</button>
        `;c(e,this)}}r.bind("monster-info-table",o);export{v as default};
