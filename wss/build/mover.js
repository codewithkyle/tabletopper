const fs = require("fs");
const path = require("path");
const cwd = process.cwd();

(async ()=>{
    let files = await fs.promises.readdir(path.join(cwd, "uws"));
    const FileTest = new RegExp(/\.node$/);
    for (let i = files.length - 1; i >= 0; i--){
        if (!FileTest.test(files[i])){
            files.splice(i, 1);
        }
    }
    for (const file of files){
        await fs.promises.copyFile(path.join(cwd, "uws", file), path.join(cwd, "dist", "uws", file));
    }
})();
