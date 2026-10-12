import { ALERT } from "./events.js";
const input = document.querySelector("[data-music-input]");
const label = document.querySelector("[data-music-label]");
const progress = document.querySelector("[data-music-progress]");
const bar = document.querySelector("[data-music-bar]");
const percent = document.querySelector("[data-music-percent]");
if (input) {
    input.addEventListener("change", () => {
        const file = input.files?.[0];
        input.value = "";
        if (file) {
            upload(file);
        }
    });
}
function alertUser(heading, message) {
    window.dispatchEvent(
        new CustomEvent(ALERT, { detail: { heading, message } }),
    );
}
function showProgress(fraction) {
    progress.hidden = false;
    const whole = Math.round(fraction * 100);
    bar.value = whole;
    percent.textContent = `${whole}%`;
}
function lock(uploading) {
    input.disabled = uploading;
    if (uploading) {
        label.setAttribute("aria-disabled", "true");
    } else {
        label.removeAttribute("aria-disabled");
    }
}
function reset() {
    progress.hidden = true;
    bar.value = 0;
    percent.textContent = "";
    lock(false);
}
function put(url, contentType, file) {
    return new Promise((resolve, reject) => {
        const request = new XMLHttpRequest();
        request.open("PUT", url);
        request.setRequestHeader("Content-Type", contentType);
        request.upload.addEventListener("progress", (event) => {
            if (event.lengthComputable) {
                showProgress(event.loaded / event.total);
            }
        });
        request.addEventListener("load", () => {
            if (request.status >= 200 && request.status < 300) {
                resolve();
            } else {
                reject(new Error(`the bucket answered ${request.status}`));
            }
        });
        request.addEventListener("error", () => reject(new Error("the upload failed")));
        request.addEventListener("abort", () => reject(new Error("the upload was cancelled")));
        request.send(file);
    });
}
async function upload(file) {
    lock(true);
    showProgress(0);
    let started;
    try {
        const response = await fetch("/assets/music", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name: file.name, size: file.size }),
        });
        if (!response.ok) {
            const problem = await response.json().catch(() => null);
            reset();
            alertUser(
                problem?.heading ?? "Upload Failed",
                problem?.message ?? "The upload could not be started. Try again in a moment.",
            );
            return;
        }
        started = await response.json();
    } catch {
        reset();
        alertUser("Upload Failed", "The upload could not be started. Check your connection and try again.");
        return;
    }
    try {
        await put(started.url, started.contentType, file);
    } catch {
        fetch(`/assets/music/${started.id}`, { method: "DELETE" }).catch(() => {});
        reset();
        alertUser("Upload Failed", "The track did not finish uploading. Try again.");
        return;
    }
    try {
        await window.htmx.ajax("POST", `/assets/music/${started.id}/confirm`, {
            target: "#music",
            swap: "afterbegin",
        });
    } finally {
        reset();
    }
}
