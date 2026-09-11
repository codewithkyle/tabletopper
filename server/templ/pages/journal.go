package pages
type JournalEntry struct {
	ID    string
	Title string
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
	Header  CharacterHeader
	Entries []JournalEntry
	Query string
}
const journalEntriesID = "journal-entries"
type JournalEntryPageData struct {
	CharacterID string
	Header  CharacterHeader
	EntryID string
	Title   string
	Body    string
}
const JournalEntryPanel = "journal"
func journalEntryTitle(entry JournalEntry) string {
	if entry.Title == "" {
		return "Untitled entry"
	}
	return entry.Title
}
