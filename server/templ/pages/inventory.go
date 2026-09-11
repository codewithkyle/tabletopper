package pages


















type InventoryItem struct {
	ID       string
	Name     string
	Quantity string
	
	
	
	
	
	Weight      string
	Value       string
	Equipped    bool
	Description string
}





type InventoryPageData struct {
	CharacterID string
	
	
	
	
	Header CharacterHeader
	Items  []InventoryItem
}






func InventoryRowPanel(itemID string) string {
	return "inventory-" + itemID
}




func equippedItemName(item InventoryItem) string {
	if item.Name == "" {
		return "Unnamed item"
	}

	return item.Name
}
