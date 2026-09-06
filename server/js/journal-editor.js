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
// STARTERKIT IS SHIPPED WHOLE while the toolbar exposes seven controls. A
// construct outside the editor's schema is destroyed on save, silently and --
// with a one-second debounce -- permanently, seconds after a paste. StarterKit
// models code, code blocks, strike, horizontal rules, hard breaks and every
// heading level, so pasted content survives and round-trips with no button of
// its own. Trimming it to the toolbar would save about 2 KB compressed and cost
// exactly that.
import { Editor } from "@tiptap/core";
import { StarterKit } from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";

// The link dialog is a fragment in the content modal, never window.prompt --
// which is as banned here as window.confirm. It is the one modal form in the
// app with no server resource behind it: the fragment is static, the submit is
// handled below and nothing is posted.
const LINK_FRAGMENT = "/fragment/character/journal-link";

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

    // THE INITIAL CONTENT IS READ FROM THE TEXTAREA, AND ONLY FROM THERE. templ
    // escapes the text of one, so the stored markdown reaches the browser as
    // data; inlining it into a <script> block would make every entry a
    // script-injection vector.
    const editor = new Editor({
        element: mount,
        extensions: [
            StarterKit.configure({ link: { openOnClick: false } }),
            Markdown,
        ],
        content: field.value,
        contentType: "markdown",
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
