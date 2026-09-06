// The journal editor: Tiptap over the hidden textarea the entry form posts.
//
// Bundled by esbuild into public/static/journal-editor.js and loaded by the
// journal entry page alone -- 117 KB of editor has no business on the character
// list. Everything else in public/js is served exactly as authored; this is the
// one built module, which is why it lives outside that directory.
//
// TIPTAP IS NOT A FORM CONTROL, so htmx's `input` trigger cannot see it. The
// bridge is the textarea: every update writes the serialised markdown into it
// and dispatches a bubbling input event, and the form's existing
// `input delay:1s` debounce then behaves exactly as it does on every other
// panel -- same trigger, same 422 handling, same error block. There is no
// second save path.
//
// The textarea is what the page renders and what the server reads, so with this
// module absent the entry is still editable as plain markdown: the toolbar is
// hidden until the editor exists, and the textarea is hidden only once it does.
//
// IMAGES ARE THE ONE THING IN HERE THAT TALKS TO THE SERVER. Every other
// control edits the document and lets the textarea's debounce carry it; an
// image has bytes, and the bytes have to be somewhere before the markdown can
// name them. So a paste, a drop or a pick uploads first and inserts the node
// only once the server has answered with a URL -- which is why this file
// contains the one fetch in the app.
//
// That fetch is outside htmx, so the alert dialog has to be raised by hand. The
// server answers a failure with the same HX-Trigger header it writes for every
// other error; htmx would have read it and dispatched the event, and here
// raiseAlert does that instead, so a failed upload opens the dialog every other
// failure opens.
//
// STARTERKIT IS SHIPPED WHOLE while the toolbar exposes eight controls. A
// construct outside the editor's schema is destroyed on save, silently and --
// with a one-second debounce -- permanently, seconds after a paste. StarterKit
// models code, code blocks, strike, horizontal rules, hard breaks and every
// heading level, so pasted content survives and round-trips with no button of
// its own. Trimming it to the toolbar would save about 2 KB compressed and cost
// exactly that.
import { Editor } from "@tiptap/core";
import { StarterKit } from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
// Aliased, because the bare name is the browser's own Image constructor and
// shadowing that in a module this size is a trap rather than a shorthand.
import { Image as ImageNode } from "@tiptap/extension-image";

// The link dialog is a fragment in the content modal, never window.prompt --
// which is as banned here as window.confirm. It is the one modal form in the
// app with no server resource behind it: the fragment is static, the submit is
// handled below and nothing is posted.
const LINK_FRAGMENT = "/fragment/character/journal-link";

// The multipart field the upload route reads. The URL it is posted to is not a
// constant: it comes off data-journal-images, rendered by the page that already
// knows the character and the entry.
const UPLOAD_FIELD = "image";

// Toolbar commands that are one call on a chain. Keys are the values of
// data-journal-mark in the markup and the extension names isActive takes, which
// is what lets the same string drive the click and the pressed state.
const COMMANDS = {
    bold: (chain) => chain.toggleBold(),
    italic: (chain) => chain.toggleItalic(),
    blockquote: (chain) => chain.toggleBlockquote(),
    bulletList: (chain) => chain.toggleBulletList(),
    orderedList: (chain) => chain.toggleOrderedList(),
};

// The extension each button reports its pressed state from. It is a second map
// rather than the keys above because the link button is `hyperlink` in the
// markup and `link` in the schema -- and it is `hyperlink` there because
// `link` is a DaisyUI component name, and a bare lowercase word in a .templ
// file is a class-name candidate Tailwind will emit a whole family for.
const ACTIVE = {
    bold: "bold",
    italic: "italic",
    hyperlink: "link",
    blockquote: "blockquote",
    bulletList: "bulletList",
    orderedList: "orderedList",
};

// Paragraph plus h2-h4, and no h1: the entry's title is the page's only h1. A
// pasted h1 is still modelled and still saved -- it is styled by size like the
// rest -- which is why the select can show a level it cannot set.
const LEVELS = [2, 3, 4];

const root = document.querySelector("[data-journal-editor]");
if (root) {
    start(root);
}

function start(root) {
    const field = root.querySelector("textarea[data-journal-body]");
    const mount = root.querySelector("[data-journal-mount]");
    const toolbar = root.querySelector("[data-journal-toolbar]");
    const headings = root.querySelector("[data-journal-heading]");
    if (!field || !mount || !toolbar || !headings) {
        console.error("journal editor markup is incomplete; leaving the textarea");
        return;
    }

    // The upload button and the file input it stands in front of. Neither is
    // required: without them the editor still takes a paste and a drop, which
    // is how most pictures arrive, so a page that renders one and not the other
    // loses a control rather than the feature.
    const picker = root.querySelector("[data-journal-file]");
    const uploadButton = toolbar.querySelector("[data-journal-upload]");

    // Uploads in flight. It is a count and not a flag because a paste can land
    // while a drop is still going.
    let uploads = 0;

    // THE INITIAL CONTENT IS READ FROM THE TEXTAREA, AND ONLY FROM THERE. templ
    // escapes the text of one, so the stored markdown reaches the browser as
    // data; inlining it into a <script> block would make every entry a
    // script-injection vector.
    const editor = new Editor({
        element: mount,
        extensions: [
            StarterKit.configure({ link: { openOnClick: false } }),
            Markdown,
            // Both options are the extension's own defaults, set out loud
            // because both are decisions. A block image is what markdown can
            // express -- `![](src)` on its own line -- and base64 is refused
            // because a data URL in the body is an entry the size of its
            // pictures, stored in the row rather than in the bucket.
            //
            // resize is deliberately absent. Width and height do not survive
            // markdown, so handles would offer an edit the next save throws
            // away.
            ImageNode.configure({ inline: false, allowBase64: false }),
        ],
        content: field.value,
        contentType: "markdown",
        // A PASTED OR DROPPED PICTURE IS TAKEN AS A FILE, and returning true is
        // what makes that stick: a browser copying an image out of a web page
        // puts both the bytes and an <img src> HTML flavour on the clipboard,
        // and letting ProseMirror parse the HTML would leave the entry pointing
        // at somebody else's server. Handling the files here stops the HTML
        // flavour from ever being read.
        editorProps: {
            handlePaste: (view, event) => {
                const files = imageFiles(event.clipboardData);
                if (files.length === 0) {
                    return false;
                }

                upload(files, view.state.selection.from);
                return true;
            },
            // moved is ProseMirror dragging a node around inside the document,
            // which carries no files and must be left to it.
            handleDrop: (view, event, slice, moved) => {
                if (moved) {
                    return false;
                }

                const files = imageFiles(event.dataTransfer);
                if (files.length === 0) {
                    return false;
                }

                // Where it was dropped, not where the caret was.
                const pos = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
                upload(files, pos ?? view.state.selection.from);
                return true;
            },
        },
        onUpdate: () => {
            field.value = editor.getMarkdown();
            field.dispatchEvent(new Event("input", { bubbles: true }));
            sync();
        },
        onSelectionUpdate: sync,
    });

    // The three visibility swaps that make the editor additive. The page ships
    // a plain textarea and a hidden toolbar and mount, so an entry is editable
    // as markdown before this module runs, with it blocked, or with JavaScript
    // off entirely -- and none of these is a class name, because public/js is
    // not a Tailwind source and a class written here would never be emitted.
    mount.hidden = false;
    field.hidden = true;
    toolbar.hidden = false;
    sync();

    // The pressed states and the heading select follow the caret. Both are
    // attributes and a value rather than class names, because public/js is
    // deliberately not a Tailwind source -- a class written in here would never
    // be emitted. journal.css styles [aria-pressed="true"].
    function sync() {
        for (const button of toolbar.querySelectorAll("[data-journal-mark]")) {
            const name = ACTIVE[button.dataset.journalMark];
            button.setAttribute("aria-pressed", String(editor.isActive(name)));
        }

        const level = LEVELS.find((l) => editor.isActive("heading", { level: l }));
        if (level) {
            headings.value = String(level);
        } else if (editor.isActive("heading")) {
            // A pasted heading at a level the select does not offer. Showing
            // "Paragraph" for it would be a lie the next keystroke acts on, so
            // the select shows nothing instead.
            headings.selectedIndex = -1;
        } else {
            headings.value = "paragraph";
        }
    }

    toolbar.addEventListener("click", (e) => {
        const button = e.target.closest("[data-journal-mark]");
        if (!button) {
            return;
        }

        const name = button.dataset.journalMark;
        if (name === "hyperlink") {
            openLinkDialog();
            return;
        }

        const command = COMMANDS[name];
        if (command) {
            command(editor.chain().focus()).run();
        }
    });

    // The button is the affordance and the input is hidden, because a bare file
    // input cannot be styled into the toolbar. Clearing the value afterwards is
    // what lets the same file be chosen twice in a row -- change does not fire
    // for a value that has not changed.
    uploadButton?.addEventListener("click", () => picker?.click());
    picker?.addEventListener("change", () => {
        const files = imageFiles(picker);
        picker.value = "";
        if (files.length > 0) {
            upload(files, editor.state.selection.from);
        }
    });

    headings.addEventListener("change", () => {
        const chain = editor.chain().focus();
        if (headings.value === "paragraph") {
            chain.setParagraph().run();
        } else {
            chain.setHeading({ level: Number(headings.value) }).run();
        }
    });

    // Ctrl/Cmd-B and -I come with StarterKit. -K does not, because the dialog
    // it opens is this app's rather than the editor's.
    mount.addEventListener("keydown", (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
            e.preventDefault();
            openLinkDialog();
        }
    });

    // Sequential, one request at a time and each awaited. A folder dropped on
    // the editor is a queue rather than twenty parallel uploads, and the
    // pictures land in the order they were dropped.
    //
    // NOTHING IS INSERTED UNTIL THE SERVER HAS ANSWERED. There is no
    // placeholder node and no upload extension: a placeholder would be a node
    // in the document that markdown cannot express, so a debounce firing while
    // one was on screen would serialise it into the entry or throw it away.
    // The busy cursor and the disabled button are the whole of the feedback.
    async function upload(files, pos) {
        uploads += 1;
        setBusy();

        try {
            // Clamped, because the position came from a drop or from a
            // selection taken before the first upload finished, and the
            // document may be shorter than it was by then.
            let at = Math.min(pos, editor.state.doc.content.size);
            for (const file of files) {
                const src = await store(file);
                // THE FIRST FAILURE ENDS THE BATCH. store has already raised
                // the dialog, and the alert modal shows one message at a time:
                // carrying on through four more files that are about to fail
                // the same way -- over the cap, offline -- would overwrite it
                // four times and leave the last one showing.
                if (!src) {
                    return;
                }

                // The insert dispatches onUpdate, which writes the textarea and
                // starts the form's debounce -- so the save that attaches the
                // image needs nothing extra from here.
                editor.chain().focus().insertContentAt(at, { type: "image", attrs: { src, alt: "" } }).run();
                // ALT STAYS EMPTY. A pasted image is called image.png and a
                // dropped one is called whatever the camera called it; neither
                // says anything to a screen reader, and a wrong description is
                // worse than none.
                at = editor.state.selection.to;
            }
        } finally {
            uploads -= 1;
            setBusy();
        }
    }

    // store posts one file and hands back the URL the server put in Location,
    // or an empty string once it has raised the alert itself.
    async function store(file) {
        const form = new FormData();
        form.append(UPLOAD_FIELD, file);

        let response;
        try {
            response = await fetch(root.dataset.journalImages, { method: "POST", body: form });
        } catch {
            // Offline, or the tab navigating away mid-upload.
            raiseAlert({
                heading: "Upload Failed",
                message: "The image could not be uploaded. Check your connection and try again.",
            });
            return "";
        }

        if (response.status === 201) {
            const src = response.headers.get("Location");
            if (src) {
                return src;
            }
        }

        raiseAlert(triggeredAlert(response.headers.get("HX-Trigger")));
        return "";
    }

    // Busy is an attribute and the button's own disabled property, for the
    // reason pressed is an attribute: public/js is not a Tailwind source, so a
    // class name written here would never be emitted. journal.css styles
    // [aria-busy="true"].
    function setBusy() {
        root.setAttribute("aria-busy", String(uploads > 0));
        if (uploadButton) {
            uploadButton.disabled = uploads > 0;
        }
    }

    // The href under the cursor, recorded before the dialog opens: by the time
    // the fragment lands, focus is in the dialog, and reading the selection
    // then would read the field the user is typing in.
    let pending = "";

    function openLinkDialog() {
        pending = editor.getAttributes("link").href ?? "";
        window.dispatchEvent(
            new CustomEvent("modal:open", {
                detail: { url: LINK_FRAGMENT, size: "sm" },
            }),
        );
    }

    // One listener does both halves: it fills the field with the recorded href
    // and takes the submit. The fragment carries no hx-* of its own -- there is
    // nothing to post -- so this is the only thing that makes it work.
    //
    // The dialog is shared, and every other fragment that lands in it is
    // someone else's; the data-journal-link check is what keeps this from
    // reaching into one.
    const dialog = document.getElementById("content-modal");
    dialog?.addEventListener("htmx:after:swap", () => {
        const form = dialog.querySelector("[data-journal-link]");
        const input = form?.querySelector('input[name="href"]');
        if (!input) {
            return;
        }

        input.value = pending;
        // once, because the fragment is fetched fresh on every open: without it
        // the listeners would stack up one per link the writer ever inserted.
        form.addEventListener(
            "submit",
            (e) => {
                e.preventDefault();
                const href = input.value.trim();
                const chain = editor.chain().focus().extendMarkRange("link");
                // An emptied field is how a link is removed. There is no second
                // control for it, and unsetLink on a selection with no link is
                // a no-op, so the empty case is safe everywhere.
                if (href) {
                    chain.setLink({ href }).run();
                } else {
                    chain.unsetLink().run();
                }
                window.dispatchEvent(new CustomEvent("modal:close"));
            },
            { once: true },
        );
    });
}

// The files on a clipboard or a drop, images only. Taking them before anything
// else is what makes copying a picture out of a web page upload it rather than
// hot-link it -- see editorProps above.
function imageFiles(transfer) {
    return Array.from(transfer?.files ?? []).filter((file) => file.type.startsWith("image/"));
}

// THE HX-TRIGGER BRIDGE. Every error the upload route can answer with is
// written by internal/htmx, which puts the heading and the message in this
// header; htmx reads it on the responses it makes and dispatches each key as an
// event, and a plain fetch gets none of that. So the one key this cares about
// is read out here and dispatched as the event alert-modal.js is already
// listening for.
//
// ONLY THE alert KEY IS LOOKED AT, and only its two strings. Nothing else in
// the header is read and nothing in it is ever evaluated -- a response is not a
// place to take instructions from, even from this server.
function triggeredAlert(header) {
    const fallback = {
        heading: "Upload Failed",
        message: "The image could not be uploaded. Refresh the page and try again.",
    };
    if (!header) {
        return fallback;
    }

    try {
        const alert = JSON.parse(header)?.alert;
        if (typeof alert?.heading === "string" && typeof alert?.message === "string") {
            return { heading: alert.heading, message: alert.message };
        }
    } catch {
        // A header that is not JSON is a bug on the server, not something to
        // show a writer mid-sentence. The fallback says what happened.
    }

    return fallback;
}

function raiseAlert(detail) {
    window.dispatchEvent(new CustomEvent("alert", { detail }));
}
