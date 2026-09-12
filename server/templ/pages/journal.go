package pages

import "github.com/a-h/templ"

type JournalEntry struct {
	ID      string
	Title   string
	Created Timestamp
	Updated Timestamp
	Snippet JournalSnippet
}
type JournalSnippet struct {
	Before string
	Match  string
	After  string
}
type Timestamp struct {
	ISO  string
	Text string
}
type JournalPageData struct {
	CharacterID string
	RoomID      string
	Header      CharacterHeader
	Entries     []JournalEntry
	Query       string
}

const journalEntriesID = "journal-entries"

type JournalEntryPageData struct {
	CharacterID string
	RoomID      string
	Header      CharacterHeader
	EntryID     string
	Title       string
	Body        string
}

const JournalEntryPanel = "journal"

const (
	JournalWindow       = "journal"
	JournalBodyID       = "journal-window"
	JournalEditorScript = "/static/journal-editor.js"
)

func JournalWindowPath(roomID string) string {
	return "/fragment/character/journal?room=" + roomID
}
func JournalEntryWindowPath(roomID string, entryID string) string {
	return JournalWindowPath(roomID) + "&entry=" + entryID
}
func journalEntryAction(data JournalEntryPageData) string {
	return "/characters/" + data.CharacterID + "/journal/" + data.EntryID
}
func journalEntryHref(data JournalPageData, entry JournalEntry) string {
	return "/characters/" + data.CharacterID + "/edit/journal/" + entry.ID
}
func journalSearchPath(data JournalPageData) string {
	if data.RoomID != "" {
		return JournalWindowPath(data.RoomID)
	}
	return "/fragment/character/journal-entries?character=" + data.CharacterID
}
func journalOpens(roomID string, entryID string) templ.Attributes {
	return templ.Attributes{
		"hx-get":    JournalEntryWindowPath(roomID, entryID),
		"hx-target": "#" + JournalBodyID,
		"hx-swap":   "outerHTML",
	}
}
func journalRoomVals(roomID string) string {
	return `{"room":"` + roomID + `"}`
}
func journalBodyBox(data JournalEntryPageData) string {
	if data.RoomID != "" {
		return journalBodyTall
	}
	return journalBodyFill
}
func journalEntryTitle(entry JournalEntry) string {
	if entry.Title == "" {
		return "Untitled entry"
	}
	return entry.Title
}
