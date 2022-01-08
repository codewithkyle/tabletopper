const express = require('express');
const app = express();
const port = 8080;
const cwd = process.cwd();
const path = require("path");
const fs = require("fs");

const serverDir = path.join(cwd, "server");
const log = path.join(serverDir, "404.log");
if (!fs.existsSync(log)){
    fs.writeFileSync(log, "");
}

const publicDir = path.join(cwd, "public");

app.get('/', (req, res) => {
    res.sendFile(path.join(publicDir, "index.html"));
});

// 404 - keep at bottom
app.get('*', (req, res) => {
    const filePath = path.join(publicDir, req.path);
    if (fs.existsSync(filePath)){
        res.sendFile(filePath);
    } else {
        fs.writeFileSync(log, `[404] ${filePath}\n`, {flag: "a"});
        res.status(404).sendFile(path.join(publicDir, "404.html"));
    }
});

app.listen(port, () => {});