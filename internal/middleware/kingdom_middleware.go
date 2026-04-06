package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"
)

// LoadKingdom looks up the kingdom for the authenticated user and attaches a
// *db.Kingdom to the context. A nil value means the user has no kingdom yet.
// It must run after RequireAuth so a user is always present.
func LoadKingdom(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
			if !ok || user == nil {
				next.ServeHTTP(w, r)
				return
			}

			kingdom, err := queries.GetKingdomByUserID(r.Context(), user.ID)
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					log.Printf("load kingdom: %v", err)
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), contextkeys.Kingdom, &kingdom)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireKingdom runs after LoadKingdom on routes that need a kingdom to exist.
// If no kingdom is in context, it redirects the user to /kingdom to create one.
func RequireKingdom(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom); !ok {
			http.Redirect(w, r, routes.KingdomPath, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
