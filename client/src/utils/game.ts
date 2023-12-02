export function CalculateModifier(value:number): string{
    const modifier = Math.floor((value - 10) / 2);
    if (modifier >= 0){
        return `+${modifier}`;
    } else {
        return `${modifier}`;
    }
}

export function CalculateProficiencyBonus(cr:number): string{
    if (cr >= 0 && cr <=4){
        return "+2";
    } else if (cr >= 5 && cr <= 8){
        return "+3";
    } else if (cr >= 9 && cr <= 12){
        return "+4";
    } else if (cr >= 13 && cr <= 16){
        return "+5";
    } else if (cr >= 17 && cr <= 20){
        return "+6";
    } else if (cr >= 21 && cr <= 24){
        return "+7";
    } else if (cr >= 26 && cr <= 28){
        return "+8";
    } else {
        return "+9";
    }
}