export function RandomInt(min:number, max:number):number{
    return Math.floor(Math.random() * (max - min)) + min;
}

export function GenerateCode():string{
    const a = ["A", "B", "C", "D", "E", "F", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9",];
    const b = [];
    for (let i = 0; i < 4; i++){
        b.push(a[RandomInt(0, a.length - 1)]);
    }
    return b.join("").toUpperCase();
}
