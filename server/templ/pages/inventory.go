package pages

type InventoryItem struct {
	ID          string
	Name        string
	Quantity    string
	Weight      string
	Value       string
	Equipped    bool
	Description string
}
type InventoryPageData struct {
	CharacterID string
	Header      CharacterHeader
	Items       []InventoryItem
	Purse       CoinPurse
}

func InventoryRowPanel(itemID string) string {
	return "inventory-" + itemID
}
func InventoryDetailsPanel(itemID string) string {
	return "inventory-details-" + itemID
}
func inventoryDetailsButtonID(itemID string) string {
	return "inventory-details-button-" + itemID
}
func inventoryDetailsURL(characterID, itemID string) string {
	return "/fragment/character/inventory-details?character=" + characterID + "&item=" + itemID
}
func inventoryItemPath(characterID, itemID string) string {
	return "/characters/" + characterID + "/inventory/" + itemID
}
func inventoryDetailsLabel(described bool) string {
	if described {
		return "Edit item details"
	}
	return "Add item details"
}
