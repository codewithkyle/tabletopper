package controllers

import (
	"context"
	"errors"
	db "main/internal/database"
	"main/internal/queries"
	"main/internal/session"
	"main/templ/pages"
	"net/http"
)

func CharactersPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	q := queries.New(db)
	results, err := q.GetCharacters(ctx, session.UserId[:])
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	pages.Characters(session, results).Render(r.Context(), w)
}

func NewCharacterPage(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	db, err := db.Connect()
	if err != nil {
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}
	session, err := session.GetUserSessionFromCookie(r, db, ctx)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			http.Redirect(w, r, "/sign-in", http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		return
	}

	pages.NewCharacter(session).Render(r.Context(), w)
}
