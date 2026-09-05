package pages

// The inventory tab, and the one place on the character sheet where the row is
// the unit of work rather than the panel.
//
// Everywhere else a panel is a form: the whole thing posts on a debounce and the
// handler replaces a column, which works because a repeater row's identity is
// its position in the list. Inventory rows are read by a second view -- the
// Character tab renders the equipped ones -- so they are real table rows with
// real ids, and a delete-all-rewrite save would churn those ids on every
// keystroke batch. Each row posts to its own URL instead.
//
// Fields are strings for the same reason EditCharacterPageData's are: the
// controller does every conversion once, and the template does none. Weight is
// the one that carries information in being empty -- see InventoryItem.Weight.

// InventoryItem is one row, already formatted for the markup. ID is the ULID as
// a string because it lands in three attributes and a URL and never in
// arithmetic.
type InventoryItem struct {
	ID       string
	Name     string
	Quantity string
	// Weight is EMPTY, not "0", for an item nobody has weighed. The column
	// cannot tell those apart and does not need to -- a 0 lb item and an
	// unweighed one are the same thing for every purpose -- but rendering the
	// zero would put a column of "0" down the side of every inventory whose
	// owner did not look up what rope weighs.
	Weight      string
	Value       string
	Equipped    bool
	Description string
}

// InventoryPageData is deliberately not EditCharacterPageData. This page has no
// abilities, no bonuses and no spell levels to supply, and making it fill in a
// struct of thirty fields to reach the two it uses would be the same mistake
// the settings struct was.
type InventoryPageData struct {
	CharacterID string
	Items       []InventoryItem
}

// InventoryRowPanel is the error-block id a row owns. PanelFormErrors documents
// that its argument is never user input; this one carries a ULID out of the URL,
// which is safe for the narrow reason that the handler parses it as a ULID
// before anything renders -- so it is 26 characters of Crockford base32 or the
// request never got here.
func InventoryRowPanel(itemID string) string {
	return "inventory-" + itemID
}

// equippedItemName covers the row that was ticked before it was named. An empty
// entry on the sheet reads as a rendering bug rather than as an unfinished row,
// and the inventory page is where it gets fixed.
func equippedItemName(item InventoryItem) string {
	if item.Name == "" {
		return "Unnamed item"
	}

	return item.Name
}
