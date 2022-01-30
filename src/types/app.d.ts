export type ToolbarMenu = "file" | "tabletop" | "initiative" | "help" | "view" | "window" | "room";

export interface Monster {
    index:            string;
    name:             string;
    size:             string;
    type:             string;
    subtype:          string | null;
    alignment:        string;
    ac:               number;
    hp:               number;
    hitDice:          string;
    str:              number;
    dex:              number;
    con:              number;
    int:              number;
    wis:              number;
    cha:              number;
    languages:        string;
    cr:               number;
    xp:               number;
    speed:            string;
    vulnerabilities:  string | null;
    resistances:      string | null;
    immunities:       string | null;
    senses:           string;
    savingThrows:     string;
    skills:           string;
    abilities:        Ability[];
    actions:          Ability[];
    legendaryActions: Ability[];
}

export interface Ability {
    name: string;
    desc: string;
}

export interface Damage {
    [level:string]: string,
}

export interface Spell {
    index:       string;
    name:        string;
    desc:        string;
    range:       string;
    components:  string[];
    ritual:      boolean;
    duration:    string;
    castingTime: string;
    level:       number;
    attackType:  string;
    damageType:  string | null;
    damage:      Damage | null;
    school:      string;
    classes:     string[];
    subsclasses: string[];
    material:    string | null;
}
