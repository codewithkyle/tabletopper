export function setValueFromKeypath(object, keypath, value){
    if (!Array.isArray(keypath)){
        keypath = keypath.split(".");
    }
    const key = keypath[0];
    keypath.splice(0, 1);
    if (keypath.length){
        setValueFromKeypath(object[key], keypath, value);
    } else {
        object[key] = value;
    }
}

export function unsetValueFromKeypath(object, keypath){
    if (!Array.isArray(keypath)){
        keypath = keypath.split(".");
    }
    const key = keypath[0];
    keypath.splice(0, 1);
    if (keypath.length){
        unsetValueFromKeypath(object[key], keypath);
    } else {
        delete object[key];
    }
}