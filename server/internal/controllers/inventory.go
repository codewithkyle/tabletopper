package controllers

import (
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
	character, characterID, ok := a.loadCharacter(w, r)
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
		Header:      characterHeader(character),
		Items:       inventoryPageItems(items),
	}))
}
func (a *App) AddInventoryItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	characterID, ok := panelCharacterID(w, r)
	if !ok {
		return
	}
	itemID := ulid.Make()
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
	finishRow(w, r, panel, inventoryToastLabel(input.Name), "item", result, err)
}
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
func formatInventoryWeight(weight float64) string {
	if weight <= 0 {
		return ""
	}
	return strconv.FormatFloat(weight, 'f', -1, 64)
}
