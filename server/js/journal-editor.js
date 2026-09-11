import { Editor } from "@tiptap/core";
import { StarterKit } from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import { Image as ImageNode } from "@tiptap/extension-image";

const LINK_FRAGMENT = "/fragment/character/journal-link";
const UPLOAD_FIELD = "image";

const COMMANDS = {
    bold: (chain) => chain.toggleBold(),
    italic: (chain) => chain.toggleItalic(),
    blockquote: (chain) => chain.toggleBlockquote(),
    bulletList: (chain) => chain.toggleBulletList(),
    orderedList: (chain) => chain.toggleOrderedList(),
};

const ACTIVE = {
    bold: "bold",
    italic: "italic",
    hyperlink: "link",
    blockquote: "blockquote",
    bulletList: "bulletList",
    orderedList: "orderedList",
};

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

    const picker = root.querySelector("[data-journal-file]");
    const uploadButton = toolbar.querySelector("[data-journal-upload]");

    let uploads = 0;

    const editor = new Editor({
        element: mount,
        extensions: [
            StarterKit.configure({ link: { openOnClick: false } }),
            Markdown,
            ImageNode.configure({ inline: false, allowBase64: false }),
        ],
        content: field.value,
        contentType: "markdown",
        editorProps: {
            handlePaste: (view, event) => {
                const files = imageFiles(event.clipboardData);
                if (files.length === 0) {
                    return false;
                }

                upload(files, view.state.selection.from);
                return true;
            },
            handleDrop: (view, event, slice, moved) => {
                if (moved) {
                    return false;
                }

                const files = imageFiles(event.dataTransfer);
                if (files.length === 0) {
                    return false;
                }

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

    mount.hidden = false;
    field.hidden = true;
    toolbar.hidden = false;
    sync();

    function sync() {
        for (const button of toolbar.querySelectorAll("[data-journal-mark]")) {
            const name = ACTIVE[button.dataset.journalMark];
            button.setAttribute("aria-pressed", String(editor.isActive(name)));
        }

        const level = LEVELS.find((l) => editor.isActive("heading", { level: l }));
        if (level) {
            headings.value = String(level);
        } else if (editor.isActive("heading")) {
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

    mount.addEventListener("keydown", (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
            e.preventDefault();
            openLinkDialog();
        }
    });

    async function upload(files, pos) {
        uploads += 1;
        setBusy();

        try {
            let at = Math.min(pos, editor.state.doc.content.size);
            for (const file of files) {
                const src = await store(file);
                if (!src) {
                    return;
                }
                editor.chain().focus().insertContentAt(at, { type: "image", attrs: { src, alt: "" } }).run();
                at = editor.state.selection.to;
            }
        } finally {
            uploads -= 1;
            setBusy();
        }
    }

    async function store(file) {
        const form = new FormData();
        form.append(UPLOAD_FIELD, file);

        let response;
        try {
            response = await fetch(root.dataset.journalImages, { method: "POST", body: form });
        } catch {
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

    function setBusy() {
        root.setAttribute("aria-busy", String(uploads > 0));
        if (uploadButton) {
            uploadButton.disabled = uploads > 0;
        }
    }

    let pending = "";

    function openLinkDialog() {
        pending = editor.getAttributes("link").href ?? "";
        window.dispatchEvent(
            new CustomEvent("modal:open", {
                detail: { url: LINK_FRAGMENT, size: "sm" },
            }),
        );
    }

    const dialog = document.getElementById("content-modal");
    dialog?.addEventListener("htmx:after:swap", () => {
        const form = dialog.querySelector("[data-journal-link]");
        const input = form?.querySelector('input[name="href"]');
        if (!input) {
            return;
        }

        input.value = pending;
        form.addEventListener(
            "submit",
            (e) => {
                e.preventDefault();
                const href = input.value.trim();
                const chain = editor.chain().focus().extendMarkRange("link");
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

function imageFiles(transfer) {
    return Array.from(transfer?.files ?? []).filter((file) => file.type.startsWith("image/"));
}

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
    }

    return fallback;
}

function raiseAlert(detail) {
    window.dispatchEvent(new CustomEvent("alert", { detail }));
}
