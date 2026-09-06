package controllers

import (
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/prefs"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
)

// The account settings dialog and the save behind it.
//
// THE DIALOG READS THE SESSION AND NEVER THE DATABASE. The session query joins
// users, so the four settings arrive with every request already; loading them
// again here would be a round trip to fetch what the caller was handed on the
// way in. It also means the dialog and the page it opens over cannot disagree,
// because they were both rendered from the same read.
//
// THE SAVE IS A POST TO /account/settings AND NOT TO /fragment/. It is a
// mutation, so it keeps its resource URL; the fragment prefix marks GET-shaped
// representations and this returns an error block, not one.
//
// A SAVED THEME IS APPLIED TWICE, and that is not redundancy. htmx.Theme
// repaints the page the reader is looking at now, because the response only
// swapped a fragment inside the dialog and <html> still carries the old
// attribute. Every page after this one is rendered with the new value by the
// shell, from the session, with nothing running on the client.
//
// The dates already on the page are left as they are. Re-rendering them would
// mean reloading, which is the thing this avoids, and unlike the theme they are
// not what the reader is looking at while the dialog closes.

// AccountSettingsFragment is the dialog, opened by the gear on the homepage.
func (a *App) AccountSettingsFragment(w http.ResponseWriter, r *http.Request) {
	p := session.FromContext(r.Context()).Prefs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AccountSettingsFragment(accountSettingsData(p, time.Now())))
}

// SaveAccountSettings writes all four, or none of them.
//
// EVERY FIELD IS VALIDATED AGAINST THE LIST THAT OFFERED IT, and a value that
// is not on one is a rejection rather than a silent fallback. The read path
// falls back -- prefs.New does, so a column this build does not understand
// still renders a page -- but a form is the other direction: accepting a zone
// the picker cannot show would store a value the reader could never see
// selected, and could not get back to after changing anything else.
//
// The four are collected before any of them is written, so a form carrying one
// bad field changes nothing. There is no partial save to explain.
func (a *App) SaveAccountSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if !parsePanelForm(w, r, pages.AccountSettingsPanel) {
		return
	}

	updated, problems := accountSettingsInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.AccountSettingsPanel, problems)
		return
	}

	err := a.Queries.UpdateUserPreferences(ctx, queries.UpdateUserPreferencesParams{
		ID:         sess.UserID,
		Theme:      queries.UsersTheme(updated.Theme),
		Timezone:   updated.Timezone,
		DateFormat: queries.UsersDateFormat(updated.DateFormat),
		TimeFormat: queries.UsersTimeFormat(updated.TimeFormat),
	})
	if err != nil {
		slog.Error("Failed to save account settings", "error", err)
		htmx.ServerError(w)
		return
	}

	announceSettings(w, r, pages.AccountSettingsPanel, updated, "Settings saved.")
}

// AccountWelcomeFragment is the same four pickers behind a welcome message,
// opened by the homepage for an account that has never answered it.
//
// IT IS OFFERED THE DEFAULTS, because that is what the row holds -- and the
// zone picker inside it is wrapped in <zone-detect>, which preselects the
// browser's own zone if this app offers it. That is the browser detection this
// project turned down for the read path, and it is right here for the reason it
// was wrong there: nothing is stored until the reader presses Save, so it is a
// suggestion sitting in a control they are already looking at rather than a
// guess written behind their back.
func (a *App) AccountWelcomeFragment(w http.ResponseWriter, r *http.Request) {
	p := session.FromContext(r.Context()).Prefs

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.AccountWelcomeFragment(accountSettingsData(p, time.Now())))
}

// CompleteOnboarding is the welcome dialog's Save. It writes the same four
// columns SaveAccountSettings does and stamps the account as set up, in ONE
// statement -- two would have a window in which the settings landed and the
// stamp did not, and the dialog would reopen over the answer just given.
func (a *App) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if !parsePanelForm(w, r, pages.AccountWelcomePanel) {
		return
	}

	updated, problems := accountSettingsInput(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.AccountWelcomePanel, problems)
		return
	}

	err := a.Queries.CompleteOnboarding(ctx, queries.CompleteOnboardingParams{
		ID:         sess.UserID,
		Theme:      queries.UsersTheme(updated.Theme),
		Timezone:   updated.Timezone,
		DateFormat: queries.UsersDateFormat(updated.DateFormat),
		TimeFormat: queries.UsersTimeFormat(updated.TimeFormat),
	})
	if err != nil {
		slog.Error("Failed to complete onboarding", "error", err)
		htmx.ServerError(w)
		return
	}

	announceSettings(w, r, pages.AccountWelcomePanel, updated, "You are all set.")
}

// DismissOnboarding is "Not now": the account is stamped and keeps every
// default, so the dialog stops opening.
//
// IT READS NO FORM AT ALL. The button sits inside the welcome form and htmx may
// well send its values along; storing them would mean the pickers the reader
// declined to answer got saved anyway, and a preselected zone they never looked
// at would become their choice.
//
// ESCAPE IS NOT THIS. Closing the dialog any other way posts nothing and leaves
// the column NULL, so the next visit asks again -- which is the right default
// for somebody who has not answered. This button is how a reader who does not
// want to answer says so once.
func (a *App) DismissOnboarding(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r.Context())

	if err := a.Queries.DismissOnboarding(r.Context(), sess.UserID); err != nil {
		slog.Error("Failed to dismiss onboarding", "error", err)
		htmx.ServerError(w)
		return
	}

	// The toast is the handover: the dialog is not coming back, so this is the
	// one chance to say where the settings went.
	htmx.CloseModal(w)
	htmx.Toast(w, "No problem. The gear at the bottom of this page has these settings whenever you want them.")
	w.WriteHeader(http.StatusOK)
}

// announceSettings is the reply both saves share: repaint, dismiss, say so, and
// clear the error block. The block is not busywork -- the form targets it, so
// rendering it empty is what clears a complaint the previous attempt left.
func announceSettings(w http.ResponseWriter, r *http.Request, panel string, p prefs.Preferences, message string) {
	htmx.Theme(w, p.Theme.Palette())
	htmx.CloseModal(w)
	htmx.Toast(w, message)
	renderPanelBlock(w, r, panel, nil)
}

// accountSettingsInput reads the four fields and reports what it could not
// accept.
//
// The messages name the field and not the value. Every one of these came out of
// a <select> the server rendered, so a rejection here is a stale page or a
// hand-made request rather than something the reader typed wrong, and quoting
// their input back at them would explain nothing.
func accountSettingsInput(r *http.Request) (prefs.Preferences, []string) {
	var (
		p        prefs.Preferences
		problems []string
	)

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

	return p, problems
}

// accountSettingsData builds the dialog. now is passed in rather than read here
// so the labels are one instant apart from each other and a test can pin them.
func accountSettingsData(p prefs.Preferences, now time.Time) pages.AccountSettingsData {
	// The examples below are rendered in the zone the reader has SAVED, which
	// is what makes "6 Sep 2026" and "07/09/2026" both correct answers on the
	// same dialog for a reader in Sydney.
	local := now.In(p.Location())

	data := pages.AccountSettingsData{
		Theme:      string(p.Theme),
		Zone:       p.Timezone,
		DateFormat: string(p.DateFormat),
		TimeFormat: string(p.TimeFormat),
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

// themeLabel names the choice rather than the palette, because "Follow system"
// is a behaviour and "Caramellatte" is a word nobody outside this repository
// has met.
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

// dateFormatLabel is today's date written the way this option would write it.
//
// THE TWO NUMERIC ONES CARRY THEIR NOTATION AND THE OTHERS DO NOT. "6 Sep 2026"
// and "2026-09-06" say what they are; "09/06/2026" and "06/09/2026" are the
// same five characters rearranged, and on twelve days a year -- when the day
// and the month are the same number -- they render identically. The suffix is
// what stops the choice being a guess on those days.
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

// timeFormatLabel names the clock and shows it, because "12-hour" alone is a
// term of art and "2:04 PM" alone does not say what the other one would look
// like.
func timeFormatLabel(t prefs.TimeFormat, at time.Time) string {
	if t == prefs.Time24H {
		return "24-hour (" + at.Format(t.Layout()) + ")"
	}
	return "12-hour (" + at.Format(t.Layout()) + ")"
}
