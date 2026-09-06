package pages

// The journal tab: a list of entries per character, and an editor page per
// entry. Entries are rows in a table of their own, like inventory items and
// spells, because each one is its own document with its own timestamps -- a
// column on the characters row could hold one journal, not a stack of them.
//
// Fields are strings for the reason every other page-data type here says: the
// controller does every conversion once and the template does none.

// JournalEntry is one row of the list. It carries no body, and the query behind
// it does not select one: an entry is kilobytes of markdown, and reading 200 of
// them to render a list of titles is most of a megabyte thrown away.
type JournalEntry struct {
	ID    string
	Title string
	// Title is EMPTY, not "Untitled entry", for an entry nobody has named. The
	// placeholder is the list's business -- see journalEntryTitle -- and
	// storing it would mean an entry that opened with that text already in the
	// field, which the writer would then have to delete before typing.
	Created Timestamp
	Updated Timestamp
}

// Timestamp is one date in the two forms the markup needs: ISO for the machine
// and Text for the reader.
//
// DATES ARE LOCALISED IN THE BROWSER, and this type is the server half of that.
// MySQL runs at +00:00 and the DSN parses times, so the controller holds a UTC
// time.Time and knows nothing whatsoever about the reader's zone -- only the
// browser does. So both forms are rendered here in UTC, and public/js/
// local-time.js rewrites the visible one in the reader's own locale and zone
// once it runs.
//
// ISO is RFC 3339, which is the machine-readable value and the input to that
// rewrite. Text is the server's UTC rendering: it is what shows before the
// module runs, with JavaScript off, and in a test.
type Timestamp struct {
	ISO  string
	Text string
}

// JournalPageData is the list. It is not EditCharacterPageData for the reason
// InventoryPageData is not: this page has no abilities, no bonuses and no spell
// levels to supply.
//
// ONE TYPE SERVES THE PAGE AND THE FRAGMENT, because a filtered list and an
// unfiltered one are the same list. The page renders it with an empty Query and
// the search route renders it with the term it matched, and both hand it to the
// same component -- a fragment is never a second copy of markup.
type JournalPageData struct {
	CharacterID string
	Entries     []JournalEntry
	// Query is the term the list was filtered by, and it exists because an
	// empty list has two meanings that need two different sentences. No entries
	// and no query is a journal nobody has written in, and the message points
	// at the button that starts one. No entries and a query is a search that
	// missed, and telling that reader to write something would be answering a
	// question they did not ask.
	Query string
}

// journalEntriesID is the list container. It is a constant because two places
// name it -- the div, and the search box's hx-target -- and a target that has
// drifted from its id fails silently: htmx finds nothing to swap and the box
// simply stops working.
//
// THE SEARCH BOX IS OUTSIDE THAT CONTAINER, and has to be. It swaps the list on
// every keystroke pause, and an element that replaces itself mid-type loses the
// caret and the value with it -- the reader would get one character per swap.
// Everything that survives a search lives above the target; only the results are
// inside it.
const journalEntriesID = "journal-entries"

// JournalEntryPageData is the editor. Body is the stored markdown, which
// reaches the browser inside a textarea and only there -- templ escapes the
// text of one, so the entry arrives as data. Inlining it into a <script> block
// instead would make every entry a script-injection vector.
type JournalEntryPageData struct {
	CharacterID string
	EntryID     string
	Title       string
	Body        string
}

// JournalEntryPanel is the error-block id the editor form owns. It is a
// constant because the page renders exactly one entry, unlike an inventory row,
// where the id has to carry the row's ULID to stay unique down a list.
const JournalEntryPanel = "journal"

// journalEntryTitle covers the entry that has not been named yet. A card with a
// blank line where its title goes reads as a rendering bug rather than as an
// unfinished entry, and a new entry is always in exactly that state: creation
// takes no fields, so an entry is born blank and titled in place.
func journalEntryTitle(entry JournalEntry) string {
	if entry.Title == "" {
		return "Untitled entry"
	}

	return entry.Title
}
