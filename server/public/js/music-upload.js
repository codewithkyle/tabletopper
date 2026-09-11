// The music upload, which is the only upload in this app the server never sees.
//
// A track is an hour or two long -- 115 to 175 MB -- so the bytes go straight
// from the file picker to R2 through a presigned URL, and this module is what
// walks that across three steps:
//
//   1. POST /assets/music with the name and size. The server checks both,
//      writes the row that claims the key, and answers with a signed URL.
//   2. PUT the file to that URL. This is the long part, and the only part with
//      anything to show, so it is an XHR rather than fetch(): XHR reports
//      upload progress and fetch() still does not.
//   3. POST .../confirm. The server looks in the bucket, checks the size and
//      the first 64 bytes, and answers with the card.
//
// STEP 3 GOES THROUGH htmx.ajax RATHER THAN fetch, so the reply is swapped into
// the grid by htmx and its HX-Trigger header is processed like any other -- the
// toast and the alert both work without this file knowing they exist.
//
// NO CLASS NAMES ARE WRITTEN HERE. server/public/js is deliberately not a
// Tailwind source, so a class named in a script is never emitted; the progress
// row is rendered by templ and toggled through the hidden attribute, and the bar
// moves by its value rather than by its width.
import { ALERT } from "./events.js";
const input = document.querySelector("[data-music-input]");
const label = document.querySelector("[data-music-label]");
const progress = document.querySelector("[data-music-progress]");
const bar = document.querySelector("[data-music-bar]");
const percent = document.querySelector("[data-music-percent]");
// Only the music page has any of this.
if (input) {
    input.addEventListener("change", () => {
        const file = input.files?.[0];
        // Clearing it now rather than at the end is what lets the same file be
        // chosen twice in a row: "change" does not fire for a value that has
        // not changed, so a failed upload could otherwise never be retried
        // without picking something else first.
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
// THE INPUT IS WHAT IS DISABLED, NOT THE LABEL. A label with aria-disabled on it
// still opens the file picker when clicked -- the attribute is advisory and
// nothing honours it -- so a second track could be started while the first was
// still going, and the two would share this one progress bar and fight over it.
// Disabling the input is what actually makes the label inert; the attribute
// stays for what announces the control.
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
// put sends the file and resolves when the bucket has it. It is a Promise
// around XHR rather than an await of fetch(), because upload progress is the
// one thing fetch() cannot report and this is the step that takes minutes.
function put(url, contentType, file) {
    return new Promise((resolve, reject) => {
        const request = new XMLHttpRequest();
        request.open("PUT", url);
        // The one header that has to be set by hand. It is part of the
        // signature, so a mismatch is refused by R2 rather than stored wrong.
        // Content-Length is set by the browser from the body.
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
        // The server's refusals arrive as the same shape the alert dialog's
        // HX-Trigger detail has, so they are dispatched rather than translated.
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
        // The row is left naming a key with nothing behind it. Asking the
        // server to drop it now is the tidy path; internal/sweep is what makes
        // it correct anyway, for the tab that was closed instead.
        fetch(`/assets/music/${started.id}`, { method: "DELETE" }).catch(() => {});
        reset();
        alertUser("Upload Failed", "The track did not finish uploading. Try again.");
        return;
    }
    // htmx owns the swap and the response headers from here. A failure inside
    // it has already raised its own alert through the HX-Trigger the server
    // sent, so there is nothing to add.
    try {
        await window.htmx.ajax("POST", `/assets/music/${started.id}/confirm`, {
            target: "#music",
            swap: "afterbegin",
        });
    } finally {
        reset();
    }
}
