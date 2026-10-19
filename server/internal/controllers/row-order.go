package controllers

import (
	"net/http"
	"strings"

	"tabletopper/internal/htmx"

	"github.com/oklog/ulid/v2"
)

func rowOrder(w http.ResponseWriter, r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		unreadableOrder(w)
		return nil, false
	}
	var ids []ulid.ULID
	for _, part := range strings.Split(r.PostFormValue("ids"), ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		id, err := ulid.Parse(part)
		if err != nil {
			unreadableOrder(w)
			return nil, false
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		unreadableOrder(w)
		return nil, false
	}
	return ids, true
}
func unreadableOrder(w http.ResponseWriter) {
	htmx.Error(w, "Order Not Saved", "The new order could not be read. Refresh the page and try again.", http.StatusBadRequest)
}
func arrange(stored, wanted []ulid.ULID) []ulid.ULID {
	left := make(map[ulid.ULID]bool, len(stored))
	for _, id := range stored {
		left[id] = true
	}
	order := make([]ulid.ULID, 0, len(stored))
	for _, id := range wanted {
		if left[id] {
			order = append(order, id)
			delete(left, id)
		}
	}
	for _, id := range stored {
		if left[id] {
			order = append(order, id)
		}
	}
	return order
}
