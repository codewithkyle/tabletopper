package pages
const shareDialogID = "share-dialog"
const ShareDialogPanel = "share-dialog"
const ShareDefaultDays = "7"
type ShareDialogData struct {
	Heading string
	Blurb   string
	Action string
	Link string
	Expires Timestamp
	Expired bool
	Protected bool
}
func journalShareDialogURL(characterID, entryID string) string {
	return "/fragment/character/journal-share?character=" + characterID + "&entry=" + entryID
}
func characterShareDialogURL(characterID string) string {
	return "/fragment/character/share?character=" + characterID
}
func monsterShareDialogURL(monsterID string) string {
	return "/fragment/monster/share?monster=" + monsterID
}
