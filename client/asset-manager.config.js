module.exports = {
    src: [
        {
            files: "./public/js/*.js",
            publicDir: "/js"
        },
        {
            files: "./public/css/*.css",
            publicDir: "/css"
        }
    ],
    output: "./public/static/service-worker-assets.js",
};
