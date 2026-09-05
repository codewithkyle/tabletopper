package controllers

import (
	"database/sql"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// The inventory tab. Every other saving thing on the sheet is a panel handler
// writing a set of columns on the characters row; these write one row of their
// own table, because the Character tab reads them back and a row referenced from
// a second view needs an identity that survives an edit.
//
// The column widths are enforced here because MySQL runs in strict mode: an
// overlong value comes back from the driver as an error, so without these a
// pasted paragraph in a 64-character field would reach the user as a 500 on a
// field they were entitled to overfill. Names and values are measured in
// characters, which is what varchar counts; the description is measured in bytes,
// which is what TEXT counts.
const (
	inventoryNameLimit        = 128
	inventoryValueLimit       = 64
	inventoryDescriptionLimit = 65535
	inventoryQuantityLimit    = 999999
	inventoryWeightLimit      = 999999.99
)

func (a *App) CharacterInventoryPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	_, characterID, ok := a.loadCharacter(w, r)
	if !ok {
		return
	}

	items, err := a.Queries.ListCharacterInventory(ctx, queries.ListCharacterInventoryParams{
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to load inventory", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.EditCharacterInventory(pages.InventoryPageData{
		CharacterID: characterID.String(),
		Items:       inventoryPageItems(items),
	}))
}

// AddInventoryItem creates an empty row and answers with it. That is the whole
// add mechanic: no counter, no index, nothing the client has to know beyond the
// character it is adding to.
//
// The row is read back rather than assembled from what the insert "should" have
// written, so the schema stays the only place a new item's starting quantity is
// declared. It costs a second round trip on a button press, which is not a
// keystroke.
//
// Answering a POST with markup is the mutation case the fragment rules name --
// the alternative is a POST that returns nothing followed by a GET to fetch what
// it made.
func (a *App) AddInventoryItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}

	itemID := ulid.Make()
	// The statement selects from characters, so a character that is not this
	// user's matches nothing and inserts nothing. Zero rows is that, and it is
	// the only thing it can be: the id is freshly minted, so a duplicate key is
	// not on the table.
	result, err := a.Queries.InsertInventoryItem(ctx, queries.InsertInventoryItemParams{
		ID:          itemID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to add inventory item", "error", err)
		htmx.ServerError(w)
		return
	}
	if inserted, err := result.RowsAffected(); err == nil && inserted == 0 {
		htmx.NotFound(w, "character")
		return
	}

	item, err := a.Queries.GetInventoryItem(ctx, queries.GetInventoryItemParams{
		ID:          itemID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to read back new inventory item", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.InventoryRow(characterID.String(), inventoryPageItem(item)))
}

// SaveInventoryItem is the row's autosave. It writes six columns and reads six
// fields, and the form that posts them renders all six together -- which is what
// makes a narrow read of a wide write impossible here. See buildInventoryInput
// for the one field where that is load-bearing rather than incidental.
func (a *App) SaveInventoryItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	itemID, ok := inventoryItemID(w, r)
	if !ok {
		return
	}

	panel := pages.InventoryRowPanel(itemID.String())
	if !parsePanelForm(w, r, panel) {
		return
	}

	input, problems := buildInventoryInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, panel, problems)
		return
	}

	result, err := a.Queries.UpdateInventoryItem(ctx, queries.UpdateInventoryItemParams{
		Name:        input.Name,
		Quantity:    input.Quantity,
		Value:       input.Value,
		Weight:      input.Weight,
		Equipped:    input.Equipped,
		Description: input.Description,
		ID:          itemID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	finishInventoryRow(w, r, panel, input.Name, result, err)
}

// DeleteInventoryItem drops one row. The reply carries no body, and it MUST be a
// 200: base.templ's noSwap config lists 204, and a status in that list sets the
// swap to "none", which overrides the hx-swap="delete" on the button and leaves
// the row sitting on screen after the database has dropped it. DeleteMap is the
// same shape for the same reason.
func (a *App) DeleteInventoryItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	itemID, ok := inventoryItemID(w, r)
	if !ok {
		return
	}

	result, err := a.Queries.DeleteInventoryItem(ctx, queries.DeleteInventoryItemParams{
		ID:          itemID,
		CharacterID: characterID,
		OwnerID:     sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to delete inventory item", "error", err)
		htmx.ServerError(w)
		return
	}
	if deleted, err := result.RowsAffected(); err == nil && deleted == 0 {
		htmx.NotFound(w, "item")
		return
	}

	htmx.Toast(w, "Item deleted.")
}

// finishInventoryRow is finishPanel's shape with one word changed, and the word
// is the reason it is not finishPanel. Zero matched rows on a panel means the
// character is not this user's; here it means this item is gone -- deleted in
// another tab, most likely -- and telling someone their character no longer
// exists because a row does would send them to look for the wrong problem.
func finishInventoryRow(w http.ResponseWriter, r *http.Request, panel string, name string, result sql.Result, err error) {
	if err != nil {
		slog.Error("Failed to save inventory item", "error", err)
		htmx.ServerError(w)
		return
	}

	if matched, err := result.RowsAffected(); err == nil && matched == 0 {
		htmx.NotFound(w, "item")
		return
	}

	htmx.Toast(w, inventoryToastLabel(name)+" saved.")
	renderPanelBlock(w, r, panel, nil)
}

// A row spends its first seconds nameless, and a debounce landing in there
// should not toast " saved.".
func inventoryToastLabel(name string) string {
	if name == "" {
		return "Item"
	}

	return name
}

type inventoryInput struct {
	Name        string
	Quantity    uint32
	Value       string
	Weight      float64
	Equipped    bool
	Description string
}

// buildInventoryInput reads one row off the form. Nothing here is required --
// specifically not the name. A row is created empty and named afterwards, so a
// required name would mean the browser refused to post the row that most needs
// posting; the repeaters learned that already, where `required` on a row made
// the drop-the-blank-row guard unreachable.
//
// EQUIPPED IS READ FROM THE ABSENCE OF A FIELD, because an unchecked box posts
// nothing at all. That is correct for a checkbox and it is also the exact shape
// the panel handlers are built to avoid -- a reader treating "not sent" as a
// value. It is safe only because the row form renders all six controls together,
// so a post that omits `equipped` really is an unticked box rather than a
// partial form. That is a property of the markup, not of this function, which is
// why there is a test pinning it.
func buildInventoryInput(r *http.Request) (inventoryInput, []string) {
	var problems []string

	name := strings.TrimSpace(r.PostFormValue("name"))
	if len([]rune(name)) > inventoryNameLimit {
		problems = append(problems, "Item name must be 128 characters or fewer.")
	}

	value := strings.TrimSpace(r.PostFormValue("value"))
	if len([]rune(value)) > inventoryValueLimit {
		problems = append(problems, "Value must be 64 characters or fewer.")
	}

	description := strings.TrimSpace(r.PostFormValue("description"))
	if len(description) > inventoryDescriptionLimit {
		problems = append(problems, "Description is too long to save.")
	}

	return inventoryInput{
		Name:        name,
		Quantity:    parseInventoryQuantity(r.PostFormValue("quantity")),
		Value:       value,
		Weight:      parseInventoryWeight(r.PostFormValue("weight")),
		Equipped:    r.PostFormValue("equipped") != "",
		Description: description,
	}, problems
}

// An empty quantity is 1, matching the column default: the field is empty for a
// moment every time someone retypes it, and a debounce landing in that moment
// should not store "none of these". Anything unparseable is 1 for a different
// reason -- type=number cannot produce it, so it is a hand-built post and not a
// user to explain anything to. A negative is floored at 0, which the field
// already declares with min="0" and is a real value: you can be out of arrows.
func parseInventoryQuantity(raw string) uint32 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 1
	}

	quantity, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 1
	}

	return uint32(min(max(quantity, 0), inventoryQuantityLimit))
}

// Weight is stored as the number it is so it can be summed, and an unweighed
// item is 0 -- there is no third state, because a 0 lb item and one nobody
// looked up are the same thing for every purpose the column has.
//
// ParseFloat accepts "NaN" and "Inf" WITHOUT an error, and DECIMAL takes
// neither. NaN fails every comparison, so it needs the explicit test; +Inf is
// caught by the ceiling and -Inf by the floor.
func parseInventoryWeight(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}

	weight, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(weight) || weight < 0 {
		return 0
	}

	return math.Min(weight, inventoryWeightLimit)
}

func inventoryItemID(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	itemID, err := ulid.Parse(r.PathValue("itemId"))
	if err != nil {
		htmx.NotFound(w, "item")
		return ulid.ULID{}, false
	}

	return itemID, true
}

func inventoryPageItems(rows []queries.Inventory) []pages.InventoryItem {
	items := make([]pages.InventoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, inventoryPageItem(row))
	}

	return items
}

func inventoryPageItem(row queries.Inventory) pages.InventoryItem {
	return pages.InventoryItem{
		ID:          row.ID.String(),
		Name:        row.Name,
		Quantity:    strconv.FormatUint(uint64(row.Quantity), 10),
		Weight:      formatInventoryWeight(row.Weight),
		Value:       row.Value,
		Equipped:    row.Equipped,
		Description: row.Description,
	}
}

// Zero renders as an empty field rather than "0" -- see InventoryItem.Weight for
// why. Precision -1 drops the column's trailing zeros, so a stored 3.00 is "3"
// and 0.50 is "0.5" instead of the two-place text DECIMAL hands back.
func formatInventoryWeight(weight float64) string {
	if weight <= 0 {
		return ""
	}

	return strconv.FormatFloat(weight, 'f', -1, 64)
}
