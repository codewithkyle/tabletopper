package controllers
import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
func (a *App) RoomInitiativeFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	view, ok := a.Hub.Initiative(ctx, row.ID, role)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomInitiative(initiativeData(row.ID, role, sess.UserID, view)))
}
func (a *App) RoomInitiativeRoundFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	view, ok := a.Hub.Initiative(ctx, row.ID, role)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	round := ""
	if len(view.Initiative.Entries) > 0 {
		round = strconv.Itoa(view.Initiative.Round)
	}
	render(w, r, pages.RoomInitiativeRound(pages.RoomInitiativeRoundData{
		RoomID:  row.ID.String(),
		Round:   round,
		Fetched: true,
	}))
}
func (a *App) RoomInitiativeEntryFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || role != room.RoleGM {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomInitiativeEntryForm(pages.RoomInitiativeEntryData{RoomID: row.ID.String()}))
}
func (a *App) SyncInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "sync the initiative tracker",
		func() (room.Command, bool) { return &room.InitiativeSync{}, true })
}
func (a *App) NextInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "advance the turn",
		func() (room.Command, bool) { return &room.InitiativeNext{}, true })
}
func (a *App) ClearInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "clear the initiative tracker",
		func() (room.Command, bool) { return &room.InitiativeClear{}, true })
}
func (a *App) OrderInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "reorder the initiative tracker", func() (room.Command, bool) {
		ids, ok := entryIDs(r)
		if !ok {
			return nil, false
		}
		return &room.InitiativeReorder{IDs: ids}, true
	})
}
func (a *App) ActivateInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "change whose turn it is", func() (room.Command, bool) {
		entry, err := ulid.Parse(r.PathValue("entry"))
		if err != nil {
			return nil, false
		}
		return &room.InitiativeActivate{Entry: entry}, true
	})
}
func (a *App) RemoveInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "remove an entry from the initiative tracker", func() (room.Command, bool) {
		entry, err := ulid.Parse(r.PathValue("entry"))
		if err != nil {
			return nil, false
		}
		return &room.InitiativeRemove{Entry: entry}, true
	})
}
func (a *App) AddInitiative(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	pawnText := strings.TrimSpace(r.FormValue("pawn"))
	if name == "" && pawnText == "" {
		renderPanelBlock(w, r, pages.RoomInitiativeEntryPanel, []string{"An entry needs a name."})
		return
	}
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}
	cmd := &room.InitiativeAdd{Name: name}
	if pawnText != "" {
		pawn, err := ulid.Parse(pawnText)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		cmd.Pawn = &pawn
	}
	if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
		var refusal *room.Error
		if pawnText == "" && errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			renderPanelBlock(w, r, pages.RoomInitiativeEntryPanel, []string{refusal.Message})
			return
		}
		a.rejectCommand(w, "add an entry to the initiative tracker", err)
		return
	}
	if pawnText == "" {
		htmx.CloseModal(w)
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) initiativeCommand(w http.ResponseWriter, r *http.Request, action string, build func() (room.Command, bool)) {
	ctx := r.Context()
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}
	cmd, ok := build()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
		a.rejectCommand(w, action, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func initiativeData(roomID ulid.ULID, role room.Role, user ulid.ULID, view *hub.InitiativeView) pages.RoomInitiativeData {
	data := pages.RoomInitiativeData{
		RoomID: roomID.String(),
		IsGM:   role == room.RoleGM,
		Empty:  len(view.Initiative.Entries) == 0,
	}
	for _, e := range view.Initiative.Entries {
		data.Entries = append(data.Entries, initiativeEntryData(role, user, view, e))
	}
	return data
}
func initiativeEntryData(role room.Role, user ulid.ULID, view *hub.InitiativeView, e room.InitiativeEntry) pages.RoomInitiativeEntry {
	out := pages.RoomInitiativeEntry{
		ID:     e.ID.String(),
		Name:   e.Name,
		Kind:   pages.EntryNamed,
		Active: view.Initiative.Active != nil && *view.Initiative.Active == e.ID,
	}
	members := make([]room.Pawn, 0, len(e.PawnIDs))
	for _, id := range e.PawnIDs {
		if p, found := view.Pawns[id]; found {
			members = append(members, p)
		}
	}
	if len(members) == 0 {
		return out
	}
	out.Name = members[0].Name
	out.Kind = pages.EntrySolo
	if len(members) > 1 {
		out.Kind = pages.EntryGroup
	}
	out.Side = string(members[0].Kind)
	for _, p := range members {
		if p.OwnerID != nil && *p.OwnerID == user {
			out.Mine = true
			break
		}
	}
	if role == room.RoleGM {
		for _, p := range members {
			if !p.Visible {
				out.Hidden = true
				break
			}
		}
	}
	face := worstHurt(members)
	out.Image = face.Image
	out.Blood = pages.InitiativeBloodVariant(face.ID.String())
	if band := room.Health(face); band != nil {
		out.Band = string(*band)
	}
	if out.Kind == pages.EntryGroup {
		bands := make([]string, 0, len(members))
		for _, p := range members {
			band := room.Health(p)
			if band == nil {
				bands = append(bands, "")
				continue
			}
			bands = append(bands, string(*band))
		}
		out.Pips, out.Count = pages.InitiativePips(bands)
		return out
	}
	solo := members[0]
	out.Solo = solo.ID.String()
	if room.ExactHP(solo.Kind, view.Table.PawnLabels, role) {
		out.HP = pages.PawnHPText(solo.HP, solo.MaxHP)
	} else if solo.HPBand != nil {
		out.HP = pages.PawnBandText(string(*solo.HPBand))
	}
	out.Conditions = pawnConditions(solo.Conditions)
	return out
}
func worstHurt(members []room.Pawn) room.Pawn {
	face := members[0]
	worst := -1
	for _, p := range members {
		rank := bandRank(room.Health(p))
		if rank == deadRank {
			continue
		}
		if rank > worst {
			worst = rank
			face = p
		}
	}
	return face
}
const deadRank = 6
func bandRank(band *room.HPBand) int {
	if band == nil {
		return 0
	}
	switch *band {
	case room.BandHealthy:
		return 1
	case room.BandBruised:
		return 2
	case room.BandBloody:
		return 3
	case room.BandVeryBloody:
		return 4
	case room.BandNearDeath:
		return 5
	case room.BandDead:
		return deadRank
	}
	return 0
}
func entryIDs(r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		return nil, false
	}
	var raw []string
	for _, value := range r.Form["entries"] {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				raw = append(raw, part)
			}
		}
	}
	if len(raw) == 0 || len(raw) > room.InitiativeMax {
		return nil, false
	}
	ids := make([]ulid.ULID, 0, len(raw))
	for _, value := range raw {
		id, err := ulid.Parse(value)
		if err != nil {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}
