package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/prefs"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
)














































func (a *App) AccountSettingsFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	data := accountSettingsData(sess.Username, sess.Prefs, time.Now())

	used, err := a.Queries.SumOwnedAssetBytes(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to total an account's storage", "error", err)
	} else {
		data.Storage = formatBytes(used)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AccountSettingsFragment(data))
}





var storageUnits = []string{"KB", "MB", "GB", "TB"}










func formatBytes(n int64) string {
	if n <= 0 {
		return "Nothing uploaded yet"
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}

	size := float64(n) / 1024
	unit := storageUnits[0]
	for _, next := range storageUnits[1:] {
		if size < 1024 {
			break
		}
		size /= 1024
		unit = next
	}

	return fmt.Sprintf("%.1f %s", size, unit)
}
















func (a *App) SaveAccountSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if !parsePanelForm(w, r, pages.AccountSettingsPanel) {
		return
	}

	name, updated, problems := accountSettingsInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.AccountSettingsPanel, problems)
		return
	}

	err := a.Queries.UpdateUserPreferences(ctx, queries.UpdateUserPreferencesParams{
		ID:         sess.UserID,
		Username:   name,
		Theme:      queries.UsersTheme(updated.Theme),
		Timezone:   updated.Timezone,
		DateFormat: queries.UsersDateFormat(updated.DateFormat),
		TimeFormat: queries.UsersTimeFormat(updated.TimeFormat),
		FollowTurn: updated.FollowTurn,
		ShowBlood:  updated.ShowBlood,
		PingVolume: uint8(updated.PingVolume),
	})
	if err != nil {
		slog.Error("Failed to save account settings", "error", err)
		htmx.ServerError(w)
		return
	}

	announceSettings(w, r, pages.AccountSettingsPanel, name, updated, "Settings saved.")
}



















func (a *App) AccountWelcomeFragment(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r.Context())
	p := sess.Prefs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AccountWelcomeFragment(accountSettingsData(sess.Username, p, time.Now())))
}





func (a *App) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if !parsePanelForm(w, r, pages.AccountWelcomePanel) {
		return
	}

	name, updated, problems := accountSettingsInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.AccountWelcomePanel, problems)
		return
	}

	err := a.Queries.CompleteOnboarding(ctx, queries.CompleteOnboardingParams{
		ID:         sess.UserID,
		Username:   name,
		Theme:      queries.UsersTheme(updated.Theme),
		Timezone:   updated.Timezone,
		DateFormat: queries.UsersDateFormat(updated.DateFormat),
		TimeFormat: queries.UsersTimeFormat(updated.TimeFormat),
		FollowTurn: updated.FollowTurn,
		ShowBlood:  updated.ShowBlood,
		PingVolume: uint8(updated.PingVolume),
	})
	if err != nil {
		slog.Error("Failed to complete onboarding", "error", err)
		htmx.ServerError(w)
		return
	}

	announceSettings(w, r, pages.AccountWelcomePanel, name, updated, "You are all set.")
}













func (a *App) DismissOnboarding(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r.Context())

	if err := a.Queries.DismissOnboarding(r.Context(), sess.UserID); err != nil {
		slog.Error("Failed to dismiss onboarding", "error", err)
		htmx.ServerError(w)
		return
	}

	
	
	htmx.CloseModal(w)
	htmx.Toast(w, "No problem. The gear at the bottom of this page has these settings whenever you want them.")
	w.WriteHeader(http.StatusOK)
}
















func announceSettings(w http.ResponseWriter, r *http.Request, panel string, name string, p prefs.Preferences, message string) {
	htmx.Theme(w, p.Theme.Palette())
	htmx.Settings(w, name, p.FollowTurn, p.ShowBlood, p.PingVolume)
	htmx.CloseModal(w)
	htmx.Toast(w, message)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.PanelFormErrors(panel, nil))
}







func accountSettingsInput(r *http.Request) (string, prefs.Preferences, []string) {
	var (
		p        prefs.Preferences
		problems []string
	)

	name, problem := accountDisplayName(r)
	if problem != "" {
		problems = append(problems, problem)
	}

	theme, ok := prefs.ParseTheme(r.PostFormValue("theme"))
	if !ok {
		problems = append(problems, "Choose one of the offered themes.")
	}
	p.Theme = theme

	zone, ok := prefs.ParseTimezone(r.PostFormValue("timezone"))
	if !ok {
		problems = append(problems, "Choose one of the offered time zones.")
	}
	p.Timezone = zone

	dateFormat, ok := prefs.ParseDateFormat(r.PostFormValue("date_format"))
	if !ok {
		problems = append(problems, "Choose one of the offered date formats.")
	}
	p.DateFormat = dateFormat

	timeFormat, ok := prefs.ParseTimeFormat(r.PostFormValue("time_format"))
	if !ok {
		problems = append(problems, "Choose either the 12-hour or the 24-hour clock.")
	}
	p.TimeFormat = timeFormat

	
	
	
	
	
	
	
	
	
	
	p.FollowTurn = r.PostFormValue("follow_turn") != ""
	p.ShowBlood = r.PostFormValue("show_blood") != ""

	
	
	
	
	
	
	pingVolume, ok := prefs.ParsePingVolume(r.PostFormValue("ping_volume"))
	if !ok {
		problems = append(problems, "Choose one of the offered ping volumes.")
	}
	p.PingVolume = pingVolume

	return name, p, problems
}












func accountDisplayName(r *http.Request) (string, string) {
	name := strings.TrimSpace(r.PostFormValue("username"))

	switch {
	case name == "":
		return "", "Enter a display name."
	case len([]rune(name)) > pages.DisplayNameLimit:
		return "", "Display name must be 128 characters or fewer."
	}

	return name, ""
}



func accountSettingsData(name string, p prefs.Preferences, now time.Time) pages.AccountSettingsData {
	
	
	
	local := now.In(p.Location())

	data := pages.AccountSettingsData{
		Name:       name,
		Theme:      string(p.Theme),
		Zone:       p.Timezone,
		DateFormat: string(p.DateFormat),
		TimeFormat: string(p.TimeFormat),
		FollowTurn: p.FollowTurn,
		ShowBlood:  p.ShowBlood,
		PingVolume: p.PingVolume,
	}

	for _, theme := range prefs.Themes() {
		data.Themes = append(data.Themes, pages.Option{
			Value: string(theme),
			Label: themeLabel(theme),
		})
	}

	for _, group := range prefs.ZoneGroups {
		zones := make([]pages.ZoneOption, 0, len(group.Zones))
		for _, zone := range group.Zones {
			zones = append(zones, pages.ZoneOption{
				Value: zone.Name,
				Label: zone.Label,
				Alias: zone.Alias,
			})
		}
		data.Zones = append(data.Zones, pages.ZoneGroup{Label: group.Region, Zones: zones})
	}

	for _, format := range prefs.DateFormats() {
		data.DateFormats = append(data.DateFormats, pages.Option{
			Value: string(format),
			Label: dateFormatLabel(format, local),
		})
	}

	for _, format := range prefs.TimeFormats() {
		data.TimeFormats = append(data.TimeFormats, pages.Option{
			Value: string(format),
			Label: timeFormatLabel(format, local),
		})
	}

	return data
}




func themeLabel(t prefs.Theme) string {
	switch t {
	case prefs.ThemeLight:
		return "Light"
	case prefs.ThemeDark:
		return "Dark"
	default:
		return "Follow system"
	}
}








func dateFormatLabel(d prefs.DateFormat, at time.Time) string {
	switch d {
	case prefs.DateMDYSlash:
		return d.Format(at) + " (MM/DD/YYYY)"
	case prefs.DateDMYSlash:
		return d.Format(at) + " (DD/MM/YYYY)"
	default:
		return d.Format(at)
	}
}




func timeFormatLabel(t prefs.TimeFormat, at time.Time) string {
	if t == prefs.Time24H {
		return "24-hour (" + at.Format(t.Layout()) + ")"
	}
	return "12-hour (" + at.Format(t.Layout()) + ")"
}
